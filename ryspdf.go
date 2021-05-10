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

// global config struct
var config Configuration

func main() {
	var err error

	// parse arguments
	var port = flag.String("port", "8080", "Port of the Rest API Server")
	flag.Parse()

	// router & server
	mux := mux.NewRouter()
	mux.HandleFunc("/stmt/{id}/{ym}", stmt)

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

	// start server
	err = srv.ListenAndServe()
	if err != nil {
		log.Fatalf("Server failed to start %v", err)
	}
}

func generate_password(acctno string) string {
	return "010180"
}

func stmt(w http.ResponseWriter, r *http.Request) {

	w.Header().Set("Content-Type", "application/pdf")

	if r.Method == "GET" {
		var err error

		vars := mux.Vars(r)
		account_no := vars["id"]
		yyyymm := vars["ym"]

		txt_file := account_no + "." + yyyymm + ".txt"
		final_pdf := cache_manager(txt_file, account_no)

		// Open file
		f, err := os.Open(final_pdf)
		if err != nil {
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
			w.WriteHeader(http.StatusInternalServerError)
		}

	}
}

func cache_manager(txt_file string, account_no string) string {
	// Check if file is generated already from previous requests
	var final_pdf string
	cache_pdf := config.PathToPdf + "/" + generate_pdf_name(txt_file)

	// if no, we need to make the PDF
	if _, err := os.Stat(cache_pdf); os.IsNotExist(err) {
		final_pdf = make_pdf(txt_file, generate_password(account_no))
	}
	// if yes, then just return the full PDF file path
	if _, err := os.Stat(cache_pdf); !os.IsNotExist(err) {
		final_pdf = cache_pdf
	}
	return final_pdf
}

func generate_pdf_name(txt_file string) string {
	// Determine PDF file name from TXT name
	file_name := strings.Split(txt_file, ".")
	final_file := file_name[0] + "." + file_name[1] + ".pdf"
	return final_file
}

func open_config(config_file string) Configuration {
	// read json config file and return as struct Configuration
	file, _ := ioutil.ReadFile(config_file)
	configuration := Configuration{}
	json.Unmarshal([]byte(file), &configuration)

	return configuration
}

func make_pdf(txt_file string, user_password string) string {
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
	pdfcontent := scan_file(config.PathToText + "/" + txt_file)

	if pdfcontent == nil {
		return ""
	}

	// Print Header defined in Config File
	for _, each_header := range config.Header {
		pdf.Cell(nil, each_header)
		pdf.Br(float64(config.FontSize))
	}

	// Print Txt each line
	for _, eachline = range pdfcontent {
		if strings.Contains(eachline, "\f") {
			pdf.AddPage()
			add_image(&pdf)
		} else {
			pdf.Cell(nil, string(eachline))
			pdf.Br(9)
		}
	}

	// Print Footer defined in config file
	for _, each_footer := range config.Footer {
		pdf.Cell(nil, each_footer)
		pdf.Br(float64(config.FontSize))
	}

	final_file := generate_pdf_name(txt_file)

	//Write PDF to physical file
	pdf.WritePdf(final_file)

	// Return the file name of PDF for HTTP Response blob write
	return final_file
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

func scan_file(path string) []string {
	file, err := os.Open(path)

	if err != nil {
		log.Fatalf("Failed to open file " + path)
		return nil
	}

	scanner := bufio.NewScanner(file)
	scanner.Split(bufio.ScanLines)
	var text []string

	for scanner.Scan() {
		text = append(text, scanner.Text())
	}

	defer file.Close()
	return text
}
