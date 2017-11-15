package views

import (
	"log"
	"net/http"

	"github.com/blit/advlight/views/assets"
	"github.com/go-chi/chi"
)

func AssetImageHandler(w http.ResponseWriter, r *http.Request) {

	imageID := chi.URLParam(r, "imageID")
	log.Printf("AssetImageHandler.%s\n", imageID)
	b, err := assets.Asset("wwwroot/img/" + imageID)
	if err != nil {
		RenderError(w, err)
		return
	}
	w.Header().Set("Content-Type", http.DetectContentType(b))
	w.Write(b)
}
