package delete

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/render"
	"log/slog"

	resp "url-shortener/internal/lib/api/response"
	"url-shortener/internal/lib/logger/sl"
	"url-shortener/internal/storage"
)

type response struct {
	resp.Response
	Url string `json:"deleted-url,omitempty"`
}

//go:generate go run github.com/vektra/mockery/v2@latest --name=URLDeleter
type URLDeleter interface {
	DeleteURL(alias string) (string, error)
}

func New(log *slog.Logger, urlDeleter URLDeleter) http.HandlerFunc { //  это фабрика хендлера.
	return func(w http.ResponseWriter, r *http.Request) {
		const op = "handlers.url.delete.New"

		log = log.With(
			slog.String("op", op),
			slog.String("request_id", middleware.GetReqID(r.Context())),
		)

		alias := chi.URLParam(r, "alias")
		if alias == "" {
			log.Info("alias is empty")

			render.Status(r, http.StatusBadRequest)
			render.JSON(w, r, resp.Error("invalid request"))
			return
		}

		deletedURL, err := urlDeleter.DeleteURL(alias)
		if errors.Is(err, storage.ErrURLNotFound) {
			log.Info("url not found", slog.String("alias", alias))

			render.Status(r, http.StatusNotFound)
			render.JSON(w, r, resp.Error("not found"))
			return
		}

		if err != nil {
			log.Error("failed to delete url", sl.Err(err))

			render.Status(r, http.StatusInternalServerError)
			render.JSON(w, r, resp.Error("internal error"))
			return
		}

		log.Info("deleted url", slog.String("url", deletedURL))

		render.Status(r, http.StatusOK)
		responseOK(w, r, deletedURL)
	}
}

func responseOK(w http.ResponseWriter, r *http.Request, deletedURL string) {
	render.JSON(w, r, response{
		Response: resp.OK(),
		Url:      deletedURL,
	})
}
