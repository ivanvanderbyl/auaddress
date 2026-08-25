package main

import (
	"archive/zip"
	"bytes"
	"testing"
)

func TestReadGNAFLocalitiesIncludesPrimaryAndAliasNames(t *testing.T) {
	var archive bytes.Buffer
	writer := zip.NewWriter(&archive)
	addZipFile(t, writer,
		"G-NAF/G-NAF AUGUST 2026/Standard/QLD_LOCALITY_psv.psv",
		"LOCALITY_PID|LOCALITY_NAME|STATE_PID\nloc1|BRISBANE|3\n",
	)
	addZipFile(t, writer,
		"G-NAF/G-NAF AUGUST 2026/Standard/ACT_LOCALITY_ALIAS_psv.psv",
		"LOCALITY_ALIAS_PID|LOCALITY_PID|NAME|STATE_PID\nalias1|loc2|CANBERRA|8\n",
	)
	if err := writer.Close(); err != nil {
		t.Fatalf("close zip writer: %v", err)
	}

	reader, err := zip.NewReader(bytes.NewReader(archive.Bytes()), int64(archive.Len()))
	if err != nil {
		t.Fatalf("open zip reader: %v", err)
	}
	localities, err := readGNAFLocalities(reader)
	if err != nil {
		t.Fatalf("read G-NAF localities: %v", err)
	}

	if got := localities["BRISBANE"]; got != stateBits["QLD"] {
		t.Errorf("BRISBANE state mask: expected %08b, got %08b", stateBits["QLD"], got)
	}
	if got := localities["CANBERRA"]; got != stateBits["ACT"] {
		t.Errorf("CANBERRA state mask: expected %08b, got %08b", stateBits["ACT"], got)
	}
}

func addZipFile(t *testing.T, writer *zip.Writer, name, contents string) {
	t.Helper()
	file, err := writer.Create(name)
	if err != nil {
		t.Fatalf("create zip file %s: %v", name, err)
	}
	if _, err := file.Write([]byte(contents)); err != nil {
		t.Fatalf("write zip file %s: %v", name, err)
	}
}
