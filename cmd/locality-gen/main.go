package main

import (
	"archive/zip"
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"go/format"
	"io"
	"net/http"
	"os"
	"path"
	"sort"
	"strconv"
	"strings"
	"time"
)

const defaultSource = "https://data.gov.au/data/dataset/19432f89-dc3a-4ef3-b943-5326ef1dbecc/resource/b023544a-5836-4d43-b6d8-da2f73e8d2bf/download/g-naf_aug26_allstates_gda2020_psv_110.zip"
const gnafPackageEndpoint = "https://data.gov.au/data/api/3/action/package_show?id=geocoded-national-address-file-g-naf"

var stateBits = map[string]uint8{
	"NSW": 1 << 0,
	"VIC": 1 << 1,
	"QLD": 1 << 2,
	"SA":  1 << 3,
	"WA":  1 << 4,
	"TAS": 1 << 5,
	"ACT": 1 << 6,
	"NT":  1 << 7,
}

func main() {
	output := flag.String("output", "localities_generated.go", "generated Go file")
	source := flag.String("source", defaultSource, "G-NAF PSV zip path or HTTP URL")
	flag.Parse()

	resolvedSource := *source
	if resolvedSource == "latest" {
		var err error
		resolvedSource, err = resolveLatestGNAF(&http.Client{Timeout: 60 * time.Second}, gnafPackageEndpoint)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}

	archive, closer, err := openZipArchive(resolvedSource)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if closer != nil {
		defer closer.Close()
	}

	localities, err := readGNAFLocalities(archive)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	generated, err := renderLocalities(localities, resolvedSource)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := os.WriteFile(*output, generated, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "write %s: %v\n", *output, err)
		os.Exit(1)
	}
}

func resolveLatestGNAF(client *http.Client, endpoint string) (string, error) {
	response, err := client.Get(endpoint)
	if err != nil {
		return "", fmt.Errorf("query latest G-NAF release: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("query latest G-NAF release: %s", response.Status)
	}

	var result struct {
		Success bool `json:"success"`
		Result  struct {
			Resources []struct {
				Name         string `json:"name"`
				Format       string `json:"format"`
				Created      string `json:"created"`
				LastModified string `json:"last_modified"`
				URL          string `json:"url"`
			} `json:"resources"`
		} `json:"result"`
	}
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode latest G-NAF release: %w", err)
	}
	if !result.Success {
		return "", fmt.Errorf("query latest G-NAF release: unsuccessful response")
	}

	latestURL := ""
	latestTime := time.Time{}
	for _, resource := range result.Result.Resources {
		name := strings.ToUpper(resource.Name)
		if !strings.Contains(name, "G-NAF") || !strings.Contains(name, "GDA2020") {
			continue
		}
		if !strings.EqualFold(resource.Format, "ZIP") && !strings.HasSuffix(strings.ToLower(resource.URL), ".zip") {
			continue
		}

		modified := parseResourceTime(resource.LastModified)
		if modified.IsZero() {
			modified = parseResourceTime(resource.Created)
		}
		if latestURL == "" || modified.After(latestTime) || modified.Equal(latestTime) && resource.URL < latestURL {
			latestURL = resource.URL
			latestTime = modified
		}
	}
	if latestURL == "" {
		return "", fmt.Errorf("query latest G-NAF release: no GDA2020 ZIP resource found")
	}
	return latestURL, nil
}

func parseResourceTime(value string) time.Time {
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return time.Time{}
	}
	return parsed
}

func openZipArchive(source string) (*zip.Reader, io.Closer, error) {
	if strings.HasPrefix(source, "https://") || strings.HasPrefix(source, "http://") {
		reader, size, err := newHTTPRangeReader(source)
		if err != nil {
			return nil, nil, err
		}
		archive, err := zip.NewReader(reader, size)
		if err != nil {
			return nil, nil, fmt.Errorf("open remote G-NAF zip: %w", err)
		}
		return archive, nil, nil
	}

	file, err := os.Open(source)
	if err != nil {
		return nil, nil, fmt.Errorf("open G-NAF zip: %w", err)
	}
	info, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, nil, fmt.Errorf("stat G-NAF zip: %w", err)
	}
	archive, err := zip.NewReader(file, info.Size())
	if err != nil {
		file.Close()
		return nil, nil, fmt.Errorf("open G-NAF zip: %w", err)
	}
	return archive, file, nil
}

func readGNAFLocalities(archive *zip.Reader) (map[string]uint8, error) {
	localities := make(map[string]uint8, 18_000)
	filesRead := 0

	for _, file := range archive.File {
		filename := path.Base(file.Name)
		state, nameColumn, ok := localityFile(filename)
		if !ok {
			continue
		}

		contents, err := file.Open()
		if err != nil {
			return nil, fmt.Errorf("open %s: %w", file.Name, err)
		}
		err = readLocalityPSV(contents, nameColumn, stateBits[state], localities)
		closeErr := contents.Close()
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", file.Name, err)
		}
		if closeErr != nil {
			return nil, fmt.Errorf("close %s: %w", file.Name, closeErr)
		}
		filesRead++
	}

	if filesRead == 0 {
		return nil, fmt.Errorf("no G-NAF locality PSV files found")
	}
	return localities, nil
}

func localityFile(filename string) (string, string, bool) {
	for state := range stateBits {
		switch filename {
		case state + "_LOCALITY_psv.psv":
			return state, "LOCALITY_NAME", true
		case state + "_LOCALITY_ALIAS_psv.psv":
			return state, "NAME", true
		}
	}
	return "", "", false
}

func readLocalityPSV(reader io.Reader, nameColumn string, bit uint8, localities map[string]uint8) error {
	records := csv.NewReader(reader)
	records.Comma = '|'
	records.FieldsPerRecord = -1
	header, err := records.Read()
	if err != nil {
		return fmt.Errorf("read header: %w", err)
	}

	nameIndex := -1
	for index, column := range header {
		if strings.EqualFold(strings.TrimSpace(column), nameColumn) {
			nameIndex = index
			break
		}
	}
	if nameIndex < 0 {
		return fmt.Errorf("missing %s column", nameColumn)
	}

	for {
		record, err := records.Read()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		if nameIndex >= len(record) {
			continue
		}
		name := normalizeLocalityName(record[nameIndex])
		if name != "" {
			localities[name] |= bit
		}
	}
}

func normalizeLocalityName(name string) string {
	return strings.ToUpper(strings.Join(strings.Fields(name), " "))
}

func renderLocalities(localities map[string]uint8, sourceName string) ([]byte, error) {
	names := make([]string, 0, len(localities))
	maxTokens := 0
	for name := range localities {
		names = append(names, name)
		if count := len(strings.Fields(name)); count > maxTokens {
			maxTokens = count
		}
	}
	sort.Strings(names)

	var source strings.Builder
	source.WriteString("// Code generated by cmd/locality-gen; DO NOT EDIT.\n")
	source.WriteString("// Source: ")
	source.WriteString(sourceName)
	source.WriteString(" (Geoscape G-NAF August 2026, GDA2020 PSV)\n\n")
	source.WriteString("package auaddress\n\n")
	fmt.Fprintf(&source, "const maxLocalityTokens = %d\n\n", maxTokens)
	source.WriteString("var localityStates = map[string]stateMask{\n")
	for _, name := range names {
		fmt.Fprintf(&source, "\t%q: 0b%08b,\n", name, localities[name])
	}
	source.WriteString("}\n")

	formatted, err := format.Source([]byte(source.String()))
	if err != nil {
		return nil, fmt.Errorf("format generated localities: %w", err)
	}
	return formatted, nil
}

type httpRangeReader struct {
	client *http.Client
	url    string
}

func newHTTPRangeReader(url string) (*httpRangeReader, int64, error) {
	client := &http.Client{Timeout: 60 * time.Second}
	response, err := client.Head(url)
	if err != nil {
		return nil, 0, fmt.Errorf("inspect remote G-NAF zip: %w", err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, 0, fmt.Errorf("inspect remote G-NAF zip: %s", response.Status)
	}
	size, err := strconv.ParseInt(response.Header.Get("Content-Length"), 10, 64)
	if err != nil || size <= 0 {
		return nil, 0, fmt.Errorf("inspect remote G-NAF zip: invalid Content-Length")
	}
	return &httpRangeReader{client: client, url: url}, size, nil
}

func (reader *httpRangeReader) ReadAt(buffer []byte, offset int64) (int, error) {
	request, err := http.NewRequest(http.MethodGet, reader.url, nil)
	if err != nil {
		return 0, err
	}
	request.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", offset, offset+int64(len(buffer))-1))
	response, err := reader.client.Do(request)
	if err != nil {
		return 0, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusPartialContent {
		return 0, fmt.Errorf("range %d-%d: %s", offset, offset+int64(len(buffer))-1, response.Status)
	}
	n, err := io.ReadFull(response.Body, buffer)
	if err == io.ErrUnexpectedEOF {
		err = io.EOF
	}
	return n, err
}
