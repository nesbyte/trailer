package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/urfave/cli/v3"
)

var Version = "development"

type CLIStruct struct {
	Port            uint16
	Bind            string
	S3Endpoint      string
	UseSSL          bool
	SecretKeyId     string
	SecretAccessKey string
}

func main() {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	// zerolog.SetGlobalLevel(zerolog.InfoLevel)

	args := CLIStruct{}

	cmd := &cli.Command{
		Name:      fmt.Sprintf("A wrapper around S3 compatible object storages - version %s", Version),
		UsageText: "Provide ACCESS_KEY_ID and SECRET_ACCESS_KEY as environment variables",
		Flags: []cli.Flag{
			&cli.Uint16Flag{
				Name:        "port",
				Value:       4132,
				Usage:       "Port to listen on",
				Destination: &args.Port,
			},
			&cli.StringFlag{
				Name:        "bind",
				Value:       "127.0.0.1",
				Usage:       "Specifies interface to bind to",
				Destination: &args.Bind,
			},
			&cli.StringFlag{
				Name:        "s3.endpoint",
				Usage:       "Which S3 endpoint to listen on",
				Required:    true,
				Destination: &args.S3Endpoint,
			},
			&cli.BoolFlag{
				Name:        "s3.useSSL",
				Usage:       "Specify to use SSL with S3",
				Value:       true,
				Destination: &args.UseSSL,
			},
		},

		Action: func(context.Context, *cli.Command) error {
			bindOn := fmt.Sprintf("%s:%d", args.Bind, args.Port)

			var ok bool
			args.SecretKeyId, ok = os.LookupEnv("ACCESS_KEY_ID")
			if !ok {
				log.Fatal().Msg("ACCESS_KEY_ID environment variable not set")
			}
			args.SecretAccessKey, ok = os.LookupEnv("SECRET_ACCESS_KEY")
			if !ok {
				log.Fatal().Msg("SECRET_ACCESS_KEY environment variable not set")
			}

			r := chi.NewRouter()
			r.Get("/v1/*", args.HandleGetObject)
			r.Get("/ping", args.HandlePing)
			log.Info().Msgf("Starting server on: '%s'", bindOn)
			return http.ListenAndServe(bindOn, r)
		},
	}

	if err := cmd.Run(context.Background(), os.Args); err != nil {
		log.Fatal().Err(err).Send()
	}
}

func (c CLIStruct) HandleGetObject(w http.ResponseWriter, r *http.Request) {

	minioClient, err := minio.New(c.S3Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(c.SecretKeyId, c.SecretAccessKey, ""),
		Secure: c.UseSSL,
	})

	if err != nil {
		msg := fmt.Sprintf("Client error setup: %s", err.Error())
		log.Error().Msg(msg)
		w.Write([]byte(msg))
		return
	}

	bucket := r.URL.Query().Get("bucket")
	if bucket == "" {
		msg := "Empty bucket provided, expecting '?bucket=your-bucket-name'"
		log.Error().Msg(msg)
		w.Write([]byte(msg))
		return
	}

	path := chi.URLParam(r, "*")
	ctx := context.Background()
	obj, err := minioClient.GetObject(ctx, bucket, path, minio.GetObjectOptions{})
	if err != nil {
		msg := fmt.Sprintf("Unable to download object: %s, from %s\n %s", "object", c.S3Endpoint, err.Error())
		log.Err(err).Msg(msg)
		return
	}
	defer obj.Close()

	info, _ := minioClient.StatObject(ctx, bucket, path, minio.StatObjectOptions{})
	w.Header().Set("Content-Length", fmt.Sprintf("%d", info.Size))

	w.Header().Set("Content-Type", "application/gzip")
	if _, err := io.Copy(w, obj); err != nil {
		msg := fmt.Sprintf("Streaming response error\n%s", err.Error())
		log.Error().Msg(msg)
		return
	}
}

func (c CLIStruct) HandlePing(w http.ResponseWriter, r *http.Request) {
	log.Debug().Msg("Received ping")
	w.Write([]byte("OK"))
}
