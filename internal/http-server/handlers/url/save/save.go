package save

import (
	resp "GoPostgres/internal/lib/api/response"
	"GoPostgres/internal/lib/random"
	"GoPostgres/internal/lib/sl"
	"GoPostgres/internal/storage"
	"errors"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/render"
	"github.com/go-playground/validator/v10"
	"io"
	"log/slog"
	"net/http"
)

type Request struct {
	URL   string `json:"Url" validate:"required, Url"`
	Alias string `json:"alias,omitempty"`
}
type Response struct {
	resp.Response
	Alias string `json:"alias,omitempty"`
}

type URLSaver interface {
	SaveUrl(urlToSave string, alias string) (int64, error)
}

const aliasLength = 6

func New(log *slog.Logger, urlSaver URLSaver) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		const op = "handlers.url.save.New"

		log = log.With(
			slog.String("op", op),
			slog.String("request_id", middleware.GetReqID(request.Context())),
		)
		var req Request

		err := render.DecodeJSON(request.Body, &request)
		if errors.Is(err, io.EOF) {
			log.Error("request body is empty")

			render.JSON(writer, request, resp.Error("empty request")) // <----

			return
		}
		if err != nil {
			log.Error("failed to decode request body", sl.Err(err))

			render.JSON(writer, request, resp.Error("failed to decode request")) // <----

			return
		}
		log.Info("request body decoded", slog.Any("req", req))

		if err := validator.New().Struct(req); err != nil {
			// Приводим ошибку к типу ошибки валидации
			validateErr := err.(validator.ValidationErrors)

			log.Error("invalid request", sl.Err(err))

			render.JSON(writer, request, resp.ValidationError(validateErr))

			return
		}

		alias := req.Alias
		if alias == "" {
			alias = random.NewRandomString(aliasLength)
		}
		id, err := urlSaver.SaveUrl(req.URL, alias)
		if errors.Is(err, storage.ErrURLExists) {
			// Отдельно обрабатываем ситуацию,
			// когда запись с таким Alias уже существует
			log.Info("url already exists", slog.String("url", req.URL))

			render.JSON(writer, request, resp.Error("url already exists"))

			return
		}
		if err != nil {
			log.Error("failed to add url", sl.Err(err))

			render.JSON(writer, request, resp.Error("failed to add url"))

			return
		}

		log.Info("url added", slog.Int64("id", id))

		responseOK(writer, request, alias)
	}
}

func responseOK(w http.ResponseWriter, r *http.Request, alias string) {
	render.JSON(w, r, Response{
		Response: resp.OK(),
		Alias:    alias,
	})
}
