// Copyright © 2011-12 Qtrac Ltd.
//
// This program or package and any associated files are licensed under the
// Apache License, Version 2.0 (the "License"); you may not use these files
// except in compliance with the License. You can get a copy of the License
// at: http://www.apache.org/licenses/LICENSE-2.0.
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	fileType             = "INVOICES"   // Used by text formats
	magicNumber          = 0x125D       // Used by binary formats
	fileVersion          = 100          // Used by all formats
	dateFormat           = "2006-01-02" // This date must always be used
	nanosecondsToSeconds = 1e9
)

type Invoice struct {
	Id         int
	CustomerId int
	Raised     time.Time
	Due        time.Time
	Paid       bool
	Note       string
	Items      []*Item
}

type Item struct {
	Id       string
	Price    float64
	Quantity int
	Note     string
}

type IInvoices interface {
	Write(writer io.Writer, invoices []*Invoice) error
	Read(reader io.Reader) ([]*Invoice, error)
}

/*
type InvoicesMarshaler interface { //запаковать
	MarshalInvoices(writer io.Writer, invoices []*Invoice) error
}

type InvoicesUnmarshaler interface { //распаковать
	UnmarshalInvoices(reader io.Reader) ([]*Invoice, error)
}
*/
func main() {
	log.SetFlags(0)
	report := false
	args := os.Args[1:]
	if len(args) > 0 && (args[0] == "-t" || args[0] == "--time") {
		report = true
		args = args[1:]
	}
	if len(args) != 2 || args[0] == "-h" || args[0] == "--help" {
		log.Fatalf("Использование: %s [-t|--time] infile.ext outfile.ext\n"+
			"-t или --time показывать время чтения/записи файла, не обязателен\n"+
			" infile.ext outfile.ext - начальный и результирующий файлы.\n"+
			" расширения файлов должны быть .gob, .inv, .jsn, .json, .txt, "+
			"или .xml, возможно в архиве gz (например .gob.gz)\n\n"+

			"usage: %s [-t|--time] infile.ext outfile.ext\n"+
			".ext may be any of .gob, .inv, .jsn, .json, .txt, "+
			"or .xml, optionally gzipped (e.g., .gob.gz)\n",
			filepath.Base(os.Args[0]), filepath.Base(os.Args[0]))
	}
	inFilename, outFilename := args[0], args[1]
	if inFilename == outFilename {
		log.Fatalln("won't overwrite a file with itself")
	}

	start := time.Now()
	invoices, err := readInvoiceFile(inFilename)
	if err == nil && report {
		duration := time.Now().Sub(start)
		fmt.Printf("Read  %s in %.3f seconds\n", inFilename,
			float64(duration)/nanosecondsToSeconds)
	}
	if err != nil {
		log.Fatalln("Failed to read:", err)
	}
	start = time.Now()
	err = writeInvoiceFile(outFilename, invoices)
	if err == nil && report {
		duration := time.Now().Sub(start)
		fmt.Printf("Wrote %s in %.3f seconds\n", outFilename,
			float64(duration)/nanosecondsToSeconds)
	}
	if err != nil {
		log.Fatalln("Failed to write:", err)
	}
}

func readInvoiceFile(filename string) ([]*Invoice, error) {
	file, closer, err := openInvoiceFile(filename)
	if closer != nil {
		defer closer()
	}
	if err != nil {
		return nil, err
	}
	return readInvoices(file, suffixOf(filename))
}

func openInvoiceFile(filename string) (io.ReadCloser, func(), error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, nil, err
	}
	closer := func() { file.Close() }
	var reader io.ReadCloser = file
	var decompressor *gzip.Reader
	if strings.HasSuffix(filename, ".gz") {
		if decompressor, err = gzip.NewReader(file); err != nil {
			return file, closer, err
		}
		closer = func() { decompressor.Close(); file.Close() }
		reader = decompressor
	}
	return reader, closer, nil
}

func readInvoices(reader io.Reader, suffix string) ([]*Invoice, error) {
	var inv IInvoices
	switch suffix {
	case ".gob":
		inv = Gob_Invoice{}
	case ".inv":
		inv = Inv_Invoice{}
	case ".jsn", ".json":
		inv = JSON_Invoice{}
	case ".txt":
		inv = Txt_Invoice{}
	case ".xml":
		inv = XML_Invoice{}
	}
	if inv != nil {
		return inv.Read(reader)
	}
	return nil, fmt.Errorf("unrecognized input suffix: %s", suffix)
}

func writeInvoiceFile(filename string, invoices []*Invoice) error {
	file, closer, err := createInvoiceFile(filename)
	if closer != nil {
		defer closer()
	}
	if err != nil {
		return err
	}
	return writeInvoices(file, suffixOf(filename), invoices)
}

func createInvoiceFile(filename string) (io.WriteCloser, func(), error) {
	file, err := os.Create(filename)
	if err != nil {
		return nil, nil, err
	}
	closer := func() { file.Close() }
	var writer io.WriteCloser = file
	var compressor *gzip.Writer
	if strings.HasSuffix(filename, ".gz") {
		compressor = gzip.NewWriter(file)
		closer = func() { compressor.Close(); file.Close() }
		writer = compressor
	}
	return writer, closer, nil
}

func writeInvoices(writer io.Writer, suffix string, invoices []*Invoice) error {
	var inv IInvoices
	switch suffix {
	case ".gob":
		inv = Gob_Invoice{}
	case ".inv":
		inv = Inv_Invoice{}
	case ".jsn", ".json":
		inv = JSON_Invoice{}
	case ".txt":
		inv = Txt_Invoice{}
	case ".xml":
		inv = XML_Invoice{}
	}
	if inv != nil {
		return inv.Write(writer, invoices)
	}
	return errors.New("unrecognized output suffix")
}

func suffixOf(filename string) string {
	suffix := filepath.Ext(filename)
	if suffix == ".gz" {
		suffix = filepath.Ext(filename[:len(filename)-3])
	}
	return suffix
}
