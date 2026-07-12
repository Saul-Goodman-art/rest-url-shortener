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

//
//go:generate go run github.com/vektra/mockery/v2@latest --name=URLDeleter
type URLDeleter interface {
	DeleteURL(alias string) (string, error)
}

func New(log *slog.Logger, urlDeleter URLDeleter) http.HandlerFunc { //  это фабрика хендлера.
	//Он создаёт и возвращает функцию‑обработчик HTTP‑запроса, в которую уже «вшиты» твой логгер и твой storage.

	// эта анонимн функция  - Это и есть HTTP‑хендлер, который Chi будет вызывать при запросе.
	return func(w http.ResponseWriter, r *http.Request) {
		const op = "handlers.url.delete.New"

		log = log.With(
			slog.String("op", op),
			slog.String("request_id", middleware.GetReqID(r.Context())),
		)

		alias := chi.URLParam(r, "alias")
		if alias == "" {
			log.Info("alias is empty")

			render.JSON(w, r, resp.Error("invalid request"))

			return
		}

		deletedURL, err := urlDeleter.DeleteURL(alias)
		if errors.Is(err, storage.ErrURLNotFound) {
			log.Info("url not found", "alias", alias)
			render.JSON(w, r, resp.Error("not found"))
			return
		}

		if err != nil {
			log.Error("failed to get url", sl.Err(err))
			render.JSON(w, r, resp.Error("internal error"))
			return
		}

		log.Info("deleted url", slog.String("url", deletedURL))

		responseOK(w, r, deletedURL)
	}
}

func responseOK(w http.ResponseWriter, r *http.Request, deletedURL string) {
	render.JSON(w, r, response{
		Response: resp.OK(),
		Url:      deletedURL,
	})
}
