# TCPWrapper

**TCPWrapper** — это Go-библиотека для обёртки TCP-соединений с поддержкой middleware, позволяющая обрабатывать запросы и ответы через настраиваемые цепочки посредников.

## Возможности

- ✅ Поддержка middleware для обработки запросов и ответов
- ✅ Автоматическое определение HTTP-ответов
- ✅ Чтение сообщений по делимитеру или заголовку Content-Length
- ✅ Контекстная поддержка (timeouts, отмена операций)
- ✅ Потокобезопасное добавление middleware
- ✅ Graceful закрытие соединений
- ✅ Встроенное логирование через zap
- ✅ Гибкая настройка через функциональные опции

## Установка

```sh
go get github.com/mmskazak/tcpwrapper
```

## Структура пакета

```
tcpwrapper/
├── tcpwrapper.go          # Основной код wrapper'а
├── options.go             # Функциональные опции настройки
├── middleware/            # Встроенные middleware
│   └── log_preview.go     # Middleware для логирования
├── is_request/            # Детекторы запросов
│   ├── contract.go        # Интерфейс IsRequestFunc
│   ├── dummy.go           # Заглушка (всегда false)
│   └── first_three_zero.go # Проверка первых трёх нулевых байт
└── is_response/           # Детекторы ответов
    ├── contract.go        # Интерфейс IsResponseFunc
    ├── dummy.go           # Заглушка (всегда false)
    └── http.go            # Проверка HTTP-ответов
```

## Быстрый старт

### Базовый пример использования

```go
package main

import (
    "context"
    "fmt"
    "net"
    "github.com/mmskazak/tcpwrapper"
    "github.com/mmskazak/tcpwrapper/middleware"
    "go.uber.org/zap"
)

func main() {
    // Создаём logger
    logger, _ := zap.NewProduction()
    
    // Слушаем TCP-порт
    listener, err := net.Listen("tcp", ":8080")
    if err != nil {
        panic(err)
    }
    defer listener.Close()

    fmt.Println("Сервер слушает на порту 8080...")

    for {
        conn, err := listener.Accept()
        if err != nil {
            fmt.Println("Ошибка принятия соединения:", err)
            continue
        }

        // Создаём wrapper с опциями
        wrapper := tcpwrapper.NewTCPWrapper(
            conn,
            tcpwrapper.WithRequestDelimiter([]byte("\n")),
            tcpwrapper.WithResponseDelimiter([]byte("\n")),
            tcpwrapper.WithLogger(logger),
            tcpwrapper.WithReadTimeout(30*time.Second),
            tcpwrapper.WithWriteTimeout(30*time.Second),
        )

        // Добавляем middleware для запросов
        wrapper.AddRequestMiddleware(
            middleware.LogMiddleware("REQUEST", 100),
        )
        
        // Добавляем middleware для ответов
        wrapper.AddResponseMiddleware(
            middleware.LogMiddleware("RESPONSE", 100),
        )

        // Обработка в горутине
        go func() {
            defer wrapper.Close()
            ctx := context.Background()
            
            // Обработать одно сообщение
            // if err := wrapper.HandleMessage(ctx); err != nil {
            //     fmt.Println("Ошибка:", err)
            // }
            
            // ИЛИ обслуживать бесконечно до отмены контекста
            if err := wrapper.Serve(ctx); err != nil {
                fmt.Println("Ошибка сервиса:", err)
            }
        }()
    }
}
```

## API

### Создание wrapper'а

```go
wrapper := tcpwrapper.NewTCPWrapper(conn net.Conn, opts ...Option) Wrapper
```

Возвращает новый экземпляр `TCPWrapper` с заданным соединением и опциями.

### Доступные опции

| Опция | Описание |
|-------|----------|
| `WithRequestDelimiter(delimiter []byte)` | Устанавливает делимитер для чтения запросов |
| `WithResponseDelimiter(delimiter []byte)` | Устанавливает делимитер для чтения ответов |
| `WithRequestChecker(checker IsRequestFunc)` | Устанавливает функцию определения запроса |
| `WithResponseChecker(checker IsResponseFunc)` | Устанавливает функцию определения ответа |
| `WithLogger(logger *zap.Logger)` | Устанавливает logger (zap) |
| `WithReadTimeout(timeout time.Duration)` | Устанавливает timeout на чтение |
| `WithWriteTimeout(timeout time.Duration)` | Устанавливает timeout на запись |
| `WithConnectionTimeout(timeout time.Duration)` | Устанавливает общий timeout на соединение |

### Методы wrapper'а

#### Добавление middleware

```go
wrapper.AddRequestMiddleware(mw Middleware)
wrapper.AddResponseMiddleware(mw Middleware)
```

Добавляет middleware в цепочку обработки. Методы потокобезопасны.

#### Обработка сообщений

```go
// Обработать одно сообщение
wrapper.HandleMessage(ctx context.Context) error

// Обслуживать бесконечно (до отмены контекста или закрытия соединения)
wrapper.Serve(ctx context.Context) error
```

#### Закрытие

```go
wrapper.Close() error
```

Закрывает соединение и освобождает ресурсы. Метод можно вызывать многократно.

### Тип Middleware

```go
type Middleware func(ctx context.Context, data []byte) ([]byte, error)
```

Middleware получает контекст и данные, возвращает обработанные данные или ошибку.

## Примеры middleware

### Логирование с превью

```go
import "github.com/mmskazak/tcpwrapper/middleware"

// Логирует первые 100 байт сообщения
mw := middleware.LogMiddleware("MY_PREFIX", 100)
wrapper.AddRequestMiddleware(mw)
```

### Конвертация кодировки (Win1251 → UTF-8)

```go
import (
    "bytes"
    "io"
    "golang.org/x/text/encoding/charmap"
    "golang.org/x/text/transform"
)

func Win1251ToUTF8(ctx context.Context, data []byte) ([]byte, error) {
    reader := transform.NewReader(
        bytes.NewReader(data), 
        charmap.Windows1251.NewDecoder(),
    )
    return io.ReadAll(reader)
}

wrapper.AddRequestMiddleware(Win1251ToUTF8)
```

### Валидация сообщения

```go
func ValidateMessage(ctx context.Context, data []byte) ([]byte, error) {
    if len(data) == 0 {
        return nil, fmt.Errorf("пустое сообщение")
    }
    // Дополнительная валидация...
    return data, nil
}

wrapper.AddRequestMiddleware(ValidateMessage)
```

## Детекторы сообщений

### Для запросов (`is_request`)

| Функция | Описание |
|---------|----------|
| `isrequest.IsDummy` | Заглушка, всегда возвращает `false` |
| `isrequest.IsFirstThreeZero` | Проверяет, что первые 3 байта равны `0x00` |

### Для ответов (`is_response`)

| Функция | Описание |
|---------|----------|
| `isresponse.IsDummy` | Заглушка, всегда возвращает `false` |
| `isresponse.IsHTTPResponse` | Проверяет, начинается ли сообщение с `HTTP/` |

### Использование собственных детекторов

```go
import "github.com/mmskazak/tcpwrapper/is_request"

// Собственная функция проверки
func MyRequestChecker(data []byte) bool {
    return bytes.HasPrefix(data, []byte("REQ:"))
}

wrapper := tcpwrapper.NewTCPWrapper(
    conn,
    tcpwrapper.WithRequestChecker(MyRequestChecker),
)
```

## Как работает чтение сообщений

Библиотека поддерживает два режима чтения:

1. **По делимитеру** — читает до указанного байта/последовательности (например, `\n`)
2. **По Content-Length** — если найден заголовок `Content-Length`, читает указанное количество байт

Приоритет: если обнаружен `Content-Length`, используется он; иначе — делимитер.

## Интеграция с Caddy

Для использования TCPWrapper с Caddy требуется создать кастомный модуль. Пример конфигурации:

```json
{
  "apps": {
    "tcp": {
      "listeners": [
        {
          "address": ":8080",
          "handler": {
            "wrapper": "tcpwrapper",
            "request_delimiter": "\n"
          }
        }
      ]
    }
  }
}
```

Запуск:

```sh
caddy run --config caddy.json
```

## Обработка ошибок и контекст

Все middleware получают `context.Context`, что позволяет:

- Отменять операции при таймауте
- Корректно завершать обработку при shutdown
- Передавать значения между middleware

Пример middleware с проверкой контекста:

```go
func SafeMiddleware(ctx context.Context, data []byte) ([]byte, error) {
    select {
    case <-ctx.Done():
        return data, ctx.Err() // Контекст отменён
    default:
        // Продолжаем обработку
    }
    
    // Ваша логика...
    return data, nil
}
```

## Лицензия

Этот проект распространяется под лицензией MIT.
