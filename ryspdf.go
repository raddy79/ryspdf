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
	PathToText           string `json:"PathToText"`
	PathToPdf            string `json:"PathToPdf"`
	FontFile             string `json:"FontFile"`
	FontSize             int    `json:"FontSize"`
	PaperSize            string `json:"PaperSize"`
	NewPageInd           string
	Header               []string `json:"Header"`
	Footer               []string `json:"Footer"`
	BgImage              []string `json:"BgImage"`
	TextVerticalOffset   float64  `json:"TextVerticalOffset"`
	TextHorizontalOffset float64  `json:"TextHorizontalOffset"`
}

// global config struct
var config Configuration

func main() {
	var err error
	var log_file *os.File

	// parse arguments
	var port = flag.String("port", "9090", "Port of the Rest API Server")
	flag.Parse()

	log_file, _ = os.OpenFile("ryspdf.log", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	defer log_file.Close()

	// router & server
	mux := mux.NewRouter()
	mux.HandleFunc("/stmt/{id}/{ym}", stmt)
	mux.HandleFunc("/stmt/{id}/{ym}/{p}", stmt)
	mux.HandleFunc("/stmt/{id}/{ym}/{p}/{f}", stmt)

	srv := &http.Server{
		Addr:         ":" + *port,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
		Handler:      mux,
	}

	// app configuration conf.json
	config = open_config("conf.json")

	log.Println("Starting Restful server at http://localhost:" + *port)
	log.SetOutput(log_file)
	log.Println("Starting Restful server at http://localhost:" + *port)

	// start server
	err = srv.ListenAndServe()
	if err != nil {
		log.Fatalf("Server failed to start %v", err)
	}
}

func stmt(w http.ResponseWriter, r *http.Request) {

	w.Header().Set("Content-Type", "application/pdf")

	if r.Method == "GET" {
		var err error
		vars := mux.Vars(r)

		account_no := vars["id"]
		yyyymm := vars["ym"]
		pdf_password := vars["p"]
		force_nocache := vars["f"]

		txt_file := account_no + "." + yyyymm + ".TXT"
		final_pdf := cache_manager(txt_file, account_no, pdf_password, force_nocache, yyyymm)

		// Open file
		f, err := os.Open(final_pdf)
		if err != nil {
			log.Println("Open Failed : " + final_pdf)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		defer f.Close()

		//Set header
		w.Header().Set("Content-type", "application/pdf")

		//Stream to response
		if _, err := io.Copy(w, f); err != nil {
			log.Println(err)
			w.WriteHeader(http.StatusInternalServerError)
		}

	}
}

func cache_manager(txt_file string, account_no string, pdf_password string, force_nocache string, yyyymm string) string {
	// Check if file is generated already from previous requests
	var final_pdf string
	cache_pdf := config.PathToPdf + "/" + generate_pdf_name(txt_file)

	// if no, we need to make the PDF
	if _, err := os.Stat(cache_pdf); os.IsNotExist(err) {
		log.Printf("Generate PDF %s", cache_pdf)
		final_pdf, err = make_pdf(txt_file, pdf_password)

		if err != nil {
			log.Printf("Make PDF error : %v", err)
		}
	}
	// if yes, then just return the full PDF file path
	if _, err := os.Stat(cache_pdf); !os.IsNotExist(err) {
		log.Printf("Cache PDF %v", cache_pdf)
		final_pdf = cache_pdf
	}

	// or, when the user specifies "nocache" it will force system to regenerate PDF
	if force_nocache == "nocache" {
		log.Printf("Nocache flag detected, regenerating %v", cache_pdf)
		var err error
		final_pdf, err = make_pdf(txt_file, pdf_password)
		if err != nil {
			log.Printf("Force Make PDF error : %v", err)
		}
	}
	return final_pdf
}

func generate_pdf_name(txt_file string) string {
	// Determine PDF file name from TXT name
	txt_name := strings.Split(txt_file, ".")
	pdf_name := txt_name[0] + "." + txt_name[1] + ".pdf"
	return pdf_name
}

func open_config(config_file string) Configuration {
	var err error
	// read json config file and return as struct Configuration
	file, err := ioutil.ReadFile(config_file)
	if err != nil {
		log.Fatalf("conf.json must be in the same directory")
	}
	configuration := Configuration{}
	json.Unmarshal([]byte(file), &configuration)

	return configuration
}

func make_pdf(txt_file string, pdf_password string) (string, error) {
	var err error
	var eachline string

	pdf := gopdf.GoPdf{}

	// Page Size and Password Protection
	pdf.Start(gopdf.Config{
		PageSize: *gopdf.PageSizeA4, //595.28, 841.89 = A4
		Protection: gopdf.PDFProtectionConfig{
			UseProtection: true,
			Permissions:   gopdf.PermissionsPrint | gopdf.PermissionsCopy | gopdf.PermissionsModify,
			OwnerPass:     []byte("fds123"),
			UserPass:      []byte(pdf_password)},
	})
	// Compression
	pdf.SetCompressLevel(1)

	// First Page and Vertical Offset
	pdf.AddPage()
	pdf.SetY(config.TextVerticalOffset)

	// Add font to PDF
	err = pdf.AddTTFFont("myfont", config.FontFile)
	if err != nil {
		log.Println("FontFile not found : " + config.FontFile)
	}

	// Add Background image of first page
	add_image(&pdf)

	// Set Font
	err = pdf.SetFont("myfont", "", config.FontSize)
	if err != nil {
		log.Println("FontSize error : " + strconv.Itoa(config.FontSize))
	}

	// Read account statement text file
	txt_name := strings.Split(txt_file, ".")
	yyyymm := txt_name[1]
	pdfcontent, err := scan_file(config.PathToText + "/" + yyyymm + "/" + txt_file)

	if err != nil {
		log.Printf("Text file read error %s %v", config.PathToText+"/"+txt_file, err)
		return "", err
	}

	// Print Header defined in Config File
	for _, each_header := range config.Header {
		pdf.Cell(nil, each_header)
		pdf.Br(float64(config.FontSize))
	}

	// Print Txt each line
	pageno := 0
	for _, eachline = range pdfcontent {
		if strings.Contains(eachline, "\f") {

			if pageno > 0 {
				pdf.AddPage()
				add_image(&pdf)
				pdf.Br(float64(config.FontSize))

				pdf.SetY(config.TextVerticalOffset + 2.1*float64(config.FontSize))
				pageno++
			}

		} else {
			pdf.SetX(config.TextHorizontalOffset)
			pdf.Cell(nil, string(eachline))
			pdf.Br(float64(config.FontSize))
			pageno++
		}
	}

	// Print Footer defined in config file
	for _, each_footer := range config.Footer {
		pdf.Cell(nil, each_footer)
		pdf.Br(float64(config.FontSize))
	}

	final_file := config.PathToPdf + "/" + generate_pdf_name(txt_file)

	//Write PDF to physical file
	pdf.WritePdf(final_file)

	// Return the file name of PDF for HTTP Response blob write
	return final_file, err
}

func add_image(pdf *gopdf.GoPdf) {
	// Background image setiap halaman
	if config.BgImage[0] != "" {
		if xx, err := strconv.ParseFloat(config.BgImage[1], 64); err == nil {
			if yy, err := strconv.ParseFloat(config.BgImage[2], 64); err == nil {
				err = pdf.Image(config.BgImage[0], xx, yy, nil)
				if err != nil {
					log.Println("BgImage error : " + config.BgImage[0])
				}
			}
		}
	}
}

func scan_file(path string) ([]string, error) {
	file, err := os.Open(path)

	if err != nil {
		//		log.Fatalf("Failed to open file " + path)
		return nil, err
	}

	scanner := bufio.NewScanner(file)
	scanner.Split(bufio.ScanLines)
	var text []string

	for scanner.Scan() {
		text = append(text, scanner.Text())
	}

	defer file.Close()
	return text, err
}
