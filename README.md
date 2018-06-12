# go-teemoproject
Golang wrapper to launch multiple [wkhtmltopdf](https://wkhtmltopdf.org/) binaries.
It uses [redis](https://redis.io) as broker.

## Build

To build the go go-teemoproject binary:
```
go build .
```

## Configuration

The following environment variables are available:

|   Environment variable  |                                    default                                    |
|:-----------------------:|:-----------------------------------------------------------------------------:|
|     `REDIS_ADDRESS`     |                               `"localhost:6379"`                              |
|      `REDIS_QUEUE`      |                               `"go-teemoproject"`                             |
|    `REDIS_PASSWORD`     |                                      `""`                                     |
|       `REDIS_DB`        |                                      `0`                                      |
| `WKHTMLTODPF_DIRECTORY` |                            `"/var/go-teemoproject"`                           |
|   `WKHTMLTODPF_PATH`    |                                `"wkhtmltopdf"`                                |
|  `WKHTMLTOPDF_OPTIONS`  | `"--quiet --enable-external-links --print-media-type --javascript-delay 300"` |


## Development

A Dockerfile and a docker-compose.yml are available for development purposes:
```
docker build -t go-teemoproject .
```
or
```
docker-compose up
```

## Run

Push urls to the queue `REDIS_QUEUE` for them to be converted

redis-cli:
```
$ redis-cli rpush go-teemoproject www.github.com
```

go example:
```
package main

import "github.com/go-redis/redis"

func main() {
    client := redis.NewClient(&redis.Options{
        Addr:     "localhost:6379",
        Password: "", // no password set
        DB:       0,  // use default DB
    })

    if err := client.RPush("go-teemoproject", "www.github.com").Err(); err != nil {
        panic(err)
    }
}
```
