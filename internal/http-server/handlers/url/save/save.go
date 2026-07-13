package save

import (
	"errors"
	"io"
	"net/http"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/render"
	"github.com/go-playground/validator/v10"

	//"golang.org/x/exp/slog"
	"log/slog" // ✅ меняем на стандартный

	resp "url-shortener/internal/lib/api/response"
	"url-shortener/internal/lib/logger/sl"
	"url-shortener/internal/lib/random"
	"url-shortener/internal/storage"
)

/*
Что делает этот код в общих чертах
Это HTTP-обработчик для сохранения сокращённых URL-адресов.
Он принимает запрос с длинным URL, генерирует для него короткий
алиас (ссылку) и сохраняет в хранилище.
*/

/*
КАК ПРОХОДИТ ПОТОК ДАННЫХ

Клиент → POST /save
{
    "url": "https://example.com/very/long/url",
    "alias": ""                    // опционально
}
    ↓
Обработчик:
    1. Проверяет URL
    2. Генерирует алиас "x7kL9p"
    3. Сохраняет в БД
    ↓
Клиент ← Ответ
{
    "status": "ok",
    "alias": "x7kL9p"
}
*/

/*
Теги управляют сериализацией/десериализацией JSON:

	json:"url" - при парсинге JSON ищет поле с ключом "url"
	json:"alias,omitempty" - ищет поле "alias", и если оно пустое (""), то при выводе в JSON это поле будет пропущено
*/
type Request struct {
	URL   string `json:"url" validate:"required,url"` //  По валидации: поля не может быть пустым, url должен быть корректным
	Alias string `json:"alias,omitempty"`             // omitempty: если этот параметр пустой, то в итоге в джесоне он будет отсутствовать
}

type Response struct {
	resp.Response
	Alias string `json:"alias,omitempty"`
}

// TODO: move to config if needed
const aliasLength = 6

// С помощью этой команды можно генерировать моки . Этот мол будет применяться в тесте/ Прямо нажимаем на запустить прямо здесь, и появится папка с моком и URLSaver.go
//
//go:generate go run github.com/vektra/mockery/v2@latest --name=URLSaver
type URLSaver interface {
	SaveURL(urlToSave string, alias string) (int64, error)
}

func New(log *slog.Logger, urlSaver URLSaver) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		const op = "handlers.url.save.New"

		log := log.With(
			slog.String("op", op),
			slog.String("request_id", middleware.GetReqID(r.Context())),
		)

		var req Request

		err := render.DecodeJSON(r.Body, &req) // Эта строка делает декодирование JSON из тела HTTP запроса в структуру Request.
		if errors.Is(err, io.EOF) {
			// Такую ошибку встретим, если получили запрос с пустым телом.
			// Обработаем её отдельно
			log.Error("request body is empty")

			render.Status(r, http.StatusBadRequest)

			render.JSON(w, r, resp.Error("empty request")) // отправляет JSON-ответ клиенту с ошибкой. сериализует в джесон
			return                                         // Добавляем return потому что render.JSON не прерывает выполнения запроса
		}
		if err != nil {
			log.Error("failed to decode request body", sl.Err(err))

			render.Status(r, http.StatusBadRequest)
			render.JSON(w, r, resp.Error("failed to decode request"))

			return
		}

		log.Info("request body decoded", slog.Any("request", req))

		if err := validator.New().Struct(req); err != nil {
			validateErr := err.(validator.ValidationErrors)

			log.Error("invalid request", sl.Err(err))

			render.Status(r, http.StatusBadRequest)
			render.JSON(w, r, resp.ValidationError(validateErr))

			return
		}

		alias := req.Alias
		if alias == "" { // Если элиас пустой то мы генерируем его из случайных символов
			alias = random.NewRandomString(aliasLength)
		}

		id, err := urlSaver.SaveURL(req.URL, alias)
		if errors.Is(err, storage.ErrURLExists) {
			log.Info("url already exists", slog.String("url", req.URL))

			render.Status(r, http.StatusConflict)
			render.JSON(w, r, resp.Error("url already exists"))

			return
		}
		if err != nil {
			log.Error("failed to add url", sl.Err(err))

			render.Status(r, http.StatusInternalServerError)
			render.JSON(w, r, resp.Error("failed to add url"))

			return
		}

		log.Info("url added", slog.Int64("id", id))

		responseOK(w, r, alias)
	}
}

func responseOK(w http.ResponseWriter, r *http.Request, alias string) {
	render.JSON(w, r, Response{
		Response: resp.OK(),
		Alias:    alias,
	})
}
