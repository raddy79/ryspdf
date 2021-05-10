package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/signintech/gopdf"
)

type Configuration struct {
	PathToText string `json:"PathToText"`
	PathToPdf  string `json:"PathToPdf"`
	FontFile   string `json:"FontFile"`
	FontSize   int    `json:"FontSize"`
	PaperSize  string `json:"PaperSize"`
	NewPageInd string
	Header     []string `json:"Header"`
	Footer     []string `json:"Footer"`
	BgImage    []string `json:"BgImage"`
}

func main() {
	var err error
	var port = flag.String("port", "12312", "Port of the Rest API Server")
	flag.Parse()

	mux := mux.NewRouter()
	mux.HandleFunc("/stmt/{id}/{ym}", stmt)

	log.Println("Starting RESTful server at http://localhost:" + *port)

	srv := &http.Server{
		Addr:         ":" + *port,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
		Handler:      mux,
	}

	err = srv.ListenAndServe()
	if err != nil {
		log.Fatalf("Server failed to start %v", err)
	}
}

func generate_password(conf Configuration, acctno string) string {
	return "010180"
}

func stmt(w http.ResponseWriter, r *http.Request) {

	w.Header().Set("Content-Type", "application/pdf")

	if r.Method == "GET" {
		vars := mux.Vars(r)
		id := vars["id"]
		yyyymm := vars["ym"]
		var err error

		conf := open_config("conf.json")
		txt_file := id + "." + yyyymm + ".txt"

		var final_pdf string
		cache_pdf := conf.PathToPdf + "/" + generate_pdf_name(conf, txt_file)

		if _, err := os.Stat(cache_pdf); os.IsNotExist(err) {
			final_pdf = make_pdf1(conf, txt_file, generate_password(conf, id))
		}

		if _, err := os.Stat(cache_pdf); !os.IsNotExist(err) {
			final_pdf = cache_pdf
		}

		// Open file
		f, err := os.Open(final_pdf)
		if err != nil {
			// fmt.Println(err + final_pdf)
			log.Println("Open Failed : " + txt_file)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		defer f.Close()

		//Set header
		w.Header().Set("Content-type", "application/pdf")

		//Stream to response
		if _, err := io.Copy(w, f); err != nil {
			log.Println(err)
			w.WriteHeader(500)
		}

	}
	//http.Error(w, "", http.StatusBadRequest)
}

func generate_pdf_name(conf Configuration, txt_file string) string {
	// Determine PDF file name from TXT name
	file_name := strings.Split(txt_file, ".")
	final_file := file_name[0] + "." + file_name[1] + ".pdf"
	return final_file
}

func open_config(config_file string) Configuration {
	file, _ := ioutil.ReadFile(config_file)
	configuration := Configuration{}
	json.Unmarshal([]byte(file), &configuration)

	return configuration
}

func make_pdf1(conf Configuration, txt_file string, user_password string) string {
	var err error
	var eachline string

	pdf := gopdf.GoPdf{}

	// Page Size and Password Protection
	pdf.Start(gopdf.Config{
		PageSize: gopdf.Rect{W: 595.28, H: 841.89}, //595.28, 841.89 = A4
		Protection: gopdf.PDFProtectionConfig{
			UseProtection: true,
			Permissions:   gopdf.PermissionsPrint | gopdf.PermissionsCopy | gopdf.PermissionsModify,
			OwnerPass:     []byte("fds123"),
			UserPass:      []byte(user_password)},
	})
	// Compression
	pdf.SetCompressLevel(1)
	pdf.AddPage()

	// Add font to PDF
	err = pdf.AddTTFFont("myfont", conf.FontFile)
	if err != nil {
		log.Println("FontFile not found : " + conf.FontFile)
	}

	// Add Background image of first page
	add_image(&pdf, conf)

	// Set Font
	err = pdf.SetFont("myfont", "", conf.FontSize)
	if err != nil {
		log.Println("FontSize error : " + strconv.Itoa(conf.FontSize))
	}

	// Read account statement text file
	pdfcontent := scan_file(conf.PathToText + "/" + txt_file)

	if pdfcontent == nil {
		return ""
	}

	// Print Header defined in Config File
	for _, each_header := range conf.Header {
		pdf.Cell(nil, each_header)
		pdf.Br(float64(conf.FontSize))
	}

	// Print Txt each line
	for _, eachline = range pdfcontent {
		if strings.Contains(eachline, "\f") {
			pdf.AddPage()
			add_image(&pdf, conf)
		} else {
			pdf.Cell(nil, string(eachline))
			pdf.Br(9)
		}
	}

	// Print Footer defined in config file
	for _, each_footer := range conf.Footer {
		pdf.Cell(nil, each_footer)
		pdf.Br(float64(conf.FontSize))
	}

	final_file := generate_pdf_name(conf, txt_file)

	//Write PDF to physical file
	pdf.WritePdf(final_file)

	// Return the file name of PDF for HTTP Response blob write
	return final_file
}

func add_image(pdf *gopdf.GoPdf, conf Configuration) {
	// Background image setiap halaman
	if conf.BgImage[0] != "" {
		if xx, err := strconv.ParseFloat(conf.BgImage[1], 64); err == nil {
			if yy, err := strconv.ParseFloat(conf.BgImage[2], 64); err == nil {
				err = pdf.Image(conf.BgImage[0], xx, yy, nil)
				if err != nil {
					log.Println("BgImage error : " + conf.BgImage[0])
				}
			}
		}
	}
}

/*func make_pdf2(conf Configuration, txt_file string, user_password string) {
	var eachline string

	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.AddPage()

	// Page Size and Password Protection
	pdf.SetProtection(gofpdf.CnProtectPrint, "123", "abc")

	// Preprinted background image
	if conf.BgImage[0] != "" {
		//if xx, err := strconv.ParseFloat(conf.BgImage[1], 64); err == nil {
		//	if yy, err := strconv.ParseFloat(conf.BgImage[2], 64); err == nil {
		pdf.Image(conf.BgImage[0], 10, 6, 30, 0, false, "", 0, "")
		//	}
		//}
	}

	// Compression
	pdf.SetCompression(true)
	pdf.AddPage()

	pdf.SetFont("SourceCode Pro", "", 7)

	pdfcontent := scan_file(conf.PathToText + "/" + txt_file)

	for _, each_header := range conf.Header {
		pdf.Cell(10, 10, each_header)
		pdf.Ln(float64(conf.FontSize))
	}

	for _, eachline = range pdfcontent {

		if strings.Contains(eachline, "\f") {
			pdf.AddPage()
		} else {
			pdf.Cell(0, 0, string(eachline))
			pdf.Ln(9)
		}

	}

	for _, each_footer := range conf.Footer {
		pdf.Cell(10, 10, each_footer)
		pdf.Ln(float64(conf.FontSize))
	}

	file_name := strings.Split(txt_file, ".")
	pdf.OutputFileAndClose(file_name[0] + "." + file_name[1] + ".pdf")
}*/

func scan_file(path string) []string {
	file, err := os.Open(path)

	if err != nil {
		// log.Fatalf("failed to open")
		return nil
	}

	scanner := bufio.NewScanner(file)
	scanner.Split(bufio.ScanLines)
	var text []string

	for scanner.Scan() {
		text = append(text, scanner.Text())
	}

	file.Close()
	return text
}
