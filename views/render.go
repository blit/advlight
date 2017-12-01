package views

import (
	"fmt"
	"html/template"
	"io"
	"io/ioutil"
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
	tplPath := "wwwroot/templates/"
	isProduction := strings.EqualFold(os.Getenv("ADVLIGHT_ENV"), "production")
	log.Println(LoadTemplates, isProduction, tplPath)
	loader := func(name string) string {
		path := tplPath + name
		if isProduction {
			return string(assets.MustAsset(path))
		}
		b, err := ioutil.ReadFile(path)
		if err != nil {
			panic(fmt.Errorf("unable to load %s: %+v", path, err))
		}
		return string(b)
	}
	if isProduction {
		fmt.Println("using production assets: ", assets.AssetNames())
	} else {
		reloadTemplates = true
	}

	GAID := os.Getenv("ADVLIGHT_GAID")
	layout, err := template.New("layout.html").Funcs(
		template.FuncMap{
			"gaID": func() string {
				return GAID
			},
		},
	).Parse(loader("layout.html"))
	if err != nil {
		return err
	}

	for _, name := range []string{
		"index.html",
		"ticket.html",
		"admin.html",
	} {
		t, err := layout.Clone()
		if err != nil {
			return err
		}
		templates[name] = template.Must(t.Parse(loader(name)))
	}
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
		log.Printf("[ERROR] Error rendering %s: %s\n", template, err)
		RenderError(wr, err)
	}
}
