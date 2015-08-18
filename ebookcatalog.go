package ebookcatalog

import (
	"archive/zip"
	"encoding/xml"
	"errors"
	"html/template"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

type Ebookcatalog struct {
	books []Book
	*template.Template
	folder string
}

func NewEbookcatalog(html_temp string) *Ebookcatalog {
	return &Ebookcatalog{books: make([]Book, 0), Template: template.Must(template.ParseFiles(html_temp))}
}

func (bc *Ebookcatalog) UseFolder(folder string) error {
	bc.folder = filepath.ToSlash(folder)
	if err := filepath.Walk(folder, bc.extract); err != nil {
		return err
	}
	return nil
}

func (bc *Ebookcatalog) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log.Println("Serving")
	if "./"+filepath.Base(filepath.Dir(r.URL.String())) == bc.folder {
		http.StripPrefix(filepath.ToSlash(filepath.Dir(r.URL.String())), http.FileServer(http.Dir(bc.folder))).ServeHTTP(w, r)
		return
	}
	log.Println("Executing templates")
	w.Header().Set("Content-Type", "text/html")
	if err := bc.Execute(w, bc.books); err != nil {
		log.Println(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

type container struct {
	Path path `xml:"rootfiles>rootfile"`
}

type path struct {
	P string `xml:"full-path,attr"`
}

type RawBook struct {
	Title       string `xml:"metadata>title"`
	Creator     string `xml:"metadata>creator"`
	Subject     string `xml:"metadata>subject"`
	Description string `xml:"metadata>description"`
	Language    string `xml:"metadata>language"`
	Meta    	[]name `xml:"metadata>meta"`
	Items    	[]item `xml:"manifest>item"`
}

type name struct {
	Content string `xml:"content,attr"`
	Name string `xml:"name,attr"`
}
type item struct {
	Ref string `xml:"href,attr"`
	Id string `xml:"id,attr"`
}

type Book struct {
	Title       string 
	Creator     string 
	Subject     string 
	Description string 
	Language    string 
	Path        string
	Cover       string
}

func findAndRead(file string, r *zip.ReadCloser) ([]byte, error) {
	var cont *zip.File
	//Searching for file
	for _, f := range r.File {
		if f.FileHeader.Name == file {
			cont = f
			break
		}
	}
	if cont == nil {
		return nil, errors.New(file + " not found")
	}
	//Opening and reading file
	reader, err := cont.Open()
	if err != nil {
		return nil, err
	}
	data, err := ioutil.ReadAll(reader)
	if err != nil {
		return nil, err
	}
	if reader.Close() != nil {
		return nil, err
	}
	return data, nil
}

func (bc *Ebookcatalog) extract(path string, info os.FileInfo, err error) error {
	if filepath.Ext(path) != ".epub" {
		return nil
	}
	//Extracting from .zip
	r, err := zip.OpenReader(path)
	if err != nil {
		return err
	}
	defer r.Close()

	//First need to get container to find out where is .opf file
	data, err := findAndRead("META-INF/container.xml", r)
	if err != nil {
		return err
	}
	var c container
	err = xml.Unmarshal([]byte(data), &c)
	if err != nil {
		return err
	}

	//Extracting needed metadata from .opf file
	data, err = findAndRead(c.Path.P, r)
	if err != nil {
		return err
	}
	var b RawBook
	err = xml.Unmarshal([]byte(data), &b)
	if err != nil {
		return err
	}
	Path := filepath.ToSlash(path)
	
	//Cover quest
	//Searching for <meta name="cover" id="..." \> to get needed item id
	id := ""
	cover := ""
	for _, m := range b.Meta {
		if m.Name == "cover" {
			id = m.Content
			break
		}
	}
	//Looking for item with newfound id to get reference to the image itself
	for _, i := range b.Items {
		if i.Id == id {
			cover = i.Ref
			break
		}
	}
	//Go on if found image path
	if cover != "" {
		//Image path in .opf file is relative
		if filepath.Dir(c.Path.P) != "." {
			cover = filepath.Dir(c.Path.P) + "/" + cover
		}
		//Looking for image
		var cont *zip.File
		for _, f := range r.File {
			if f.FileHeader.Name == cover {
				cont = f
				break
			}
		}
		if cont == nil {
			return errors.New(cover + " not found")
		}
		//Copying cover outside of .zip with the new name
		cover = strings.TrimSuffix(Path, ".epub") + filepath.Ext(cont.FileHeader.Name)
		cov, err := os.Create(cover)
		if err != nil {
			return err
		}
		reader, err := cont.Open()
		if err != nil {
			return err
		}
		_, err = io.Copy(cov, reader)
		if err != nil {
			return err
		}
		if reader.Close() != nil {
			return err
		}
	}
			
	bc.books = append(bc.books, Book{b.Title, b.Creator, b.Subject, b.Description, b.Language, Path, cover})
	log.Println(b.Title)
	return nil
}
