package views

import (
	"fmt"
	"html/template"
	"io"
	"log"
	"os"
	"strings"

	"github.com/blit/advlight/views/assets"
)

var templates map[string]*template.Template = make(map[string]*template.Template)
var reloadTemplates = false

func init() {

	err := LoadTemplates()
	if err != nil {
		log.Fatalln(err)
	}
}

func LoadTemplates() error {
	if strings.EqualFold(os.Getenv("ADVLIGHT_ENV"), "production") {
		return loadTemplatesProd()
	} else {
		reloadTemplates = true
		return loadTemplatesDev()
	}
}

func loadTemplatesProd() error {
	fmt.Println(assets.AssetNames())
	tplPath := "wwwroot/templates/"
	layout, err := template.New("layout.html").Parse(string(assets.MustAsset(tplPath + "layout.html")))
	if err != nil {
		return err
	}

	fn := func(name string) error {
		t, err := layout.Clone()
		if err != nil {
			return err
		}
		templates[name] = template.Must(t.Parse(string(assets.MustAsset(tplPath + name))))
		return nil
	}
	for _, name := range []string{
		"index.html",
		"ticket.html",
	} {
		err := fn(name)
		if err != nil {
			return err
		}
	}
	return nil
}

func loadTemplatesDev() error {
	tplPath := "wwwroot/templates"
	layoutPath := tplPath + "/layout.html"
	fn := func(name string) error {
		t, err := template.ParseFiles(layoutPath, tplPath+"/"+name)
		if err != nil {
			return err
		}
		templates[name] = t
		return nil
	}
	for _, name := range []string{
		"index.html",
		//"ticket.html",
	} {
		err := fn(name)
		if err != nil {
			return err
		}
	}
	// no template for ticket
	t, err := template.ParseFiles(tplPath + "/ticket.html")
	if err != nil {
		return err
	}
	templates["ticket.html"] = t

	return nil
}

func RenderError(wr io.Writer, err error) {
	wr.Write([]byte(`An error occured: ` + err.Error()))
}

func Render(wr io.Writer, template string, data interface{}) {
	if reloadTemplates {
		err := LoadTemplates()
		if err != nil {
			RenderError(wr, err)
			return
		}
	}
	err := templates[template].Execute(wr, data)
	if err != nil {
		log.Panicf("[ERROR] Error rendering %s: %s\n", template, err)
	}
}
