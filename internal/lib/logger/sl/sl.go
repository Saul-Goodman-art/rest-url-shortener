package sl

import (
	"log/slog" // ✅ используем стандартный пакет
)

func Err(err error) slog.Attr {
	return slog.Attr{
		Key:   "error",
		Value: slog.StringValue(err.Error()),
	}
}
