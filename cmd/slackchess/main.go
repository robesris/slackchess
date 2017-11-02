package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"strings"
	"os"

	"github.com/gorilla/schema"
	"github.com/loganjspears/chess"
	"github.com/loganjspears/slackchess/internal/imageutil"
	"github.com/loganjspears/slackchess/internal/slack"
)

var token string
var url string

func init() {
	flag.StringVar(&token, "token", "", "slack token")
	flag.StringVar(&url, "url", "", "root url for of the server")
}

func determineListenAddress() (string, error) {
  port := os.Getenv("PORT")
  if port == "" {
    return "", fmt.Errorf("$PORT not set")
  }
  return ":" + port, nil
}

func main() {
	flag.Parse()
	if token == "" {
		log.Fatal("must set token flag")
	}
	if url == "" {
		log.Fatal("must set url flag")
	}
	slack.SetBaseURL(url)
	slack.SetStockfishPath("/go/src/github.com/loganjspears/slackchess/internal/stockfish")
	http.HandleFunc("/", logHandler(upHandler))
	http.HandleFunc("/command", logHandler(commandHandler))
	http.HandleFunc("/board/", logHandler(boardImgHandler))

	addr, err := determineListenAddress()
  if err != nil {
    log.Fatal(err)
  }

	log.Fatal(http.ListenAndServe(addr, nil))
}

func logHandler(handler http.HandlerFunc) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("%s %s %s", r.RemoteAddr, r.Method, r.URL)
		handler.ServeHTTP(w, r)
	})
}

func upHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "up")
}

func commandHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "", http.StatusNotFound)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	r.Form["text"] = r.Form["text"][1:]
	log.Printf("slice: %+v", r.Form["text"])
	log.Printf("slack slash command form %+v", r.Form)

	cmd := &slack.SlashCmd{}
	if err := schema.NewDecoder().Decode(cmd, r.Form); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if cmd.Token != token {
		log.Println(cmd.Token, token)
		http.Error(w, "invalid token", http.StatusBadRequest)
		return
	}
	resp := cmd.Response()
	log.Printf("sending response %+v", resp)
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func boardImgHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "", http.StatusNotFound)
		return
	}
	log.Println("board img handler - ", r.URL.Path)
	path := strings.TrimPrefix(r.URL.Path, "/board/")
	path = strings.TrimSuffix(path, ".png")
	path = path + " w KQkq - 0 1"

	fen, err := chess.FEN(path)
	if err != nil {
		http.Error(w, "could not parse fen "+err.Error(), http.StatusNotFound)
		return
	}
	v := r.URL.Query()
	sqs := squaresFromString(v.Get("markSquares"))
	g := chess.NewGame(fen)
	log.Println("creating image for fen ", path)
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "max-age=31536000")
	if err := imageutil.WritePNG(w, g.Position(), sqs...); err != nil {
		http.Error(w, "could not parse fen "+err.Error(), http.StatusInternalServerError)
		return
	}
}

// s must be in the format: a1,b2,c3
func squaresFromString(s string) []chess.Square {
	sqStrs := strings.Split(s, ",")
	sqs := []chess.Square{}
	for _, s := range sqStrs {
		if s == "" {
			continue
		}
		for i := 0; i < 64; i++ {
			sq := chess.Square(i)
			if sq.String() == s {
				sqs = append(sqs, sq)
				break
			}
		}
	}
	return sqs
}
