package poquer_test

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

var (
	dummyGame = &JogoEspiao{}
	tenMS     = 10 * time.Millisecond
)

func mustMakePlayerServer(t *testing.T, armazenamento poquer.ArmazenamentoJogador, partida poquer.Game) *poquer.PlayerServer {
	server, err := poquer.NewPlayerServer(armazenamento, partida)
	if err != nil {
		t.Fatal("problem creating player server", err)
	}
	return server
}

func TestGETPlayers(t *testing.T) {
	armazenamento := poquer.EsbocoDeArmazenamentoJogador{
		Pontuações: map[string]int{
			"Pepper": 20,
			"Floyd":  10,
		},
	}
	server := mustMakePlayerServer(t, &armazenamento, dummyGame)

	t.Run("retorna Pepper's pontuação", func(t *testing.T) {
		request := newGetScoreRequest("Pepper")
		response := httptest.NewRecorder()

		server.ServeHTTP(response, request)

		assertStatus(t, response, http.StatusOK)
		assertResponseBody(t, response.Body.String(), "20")
	})

	t.Run("retorna Floyd's pontuação", func(t *testing.T) {
		request := newGetScoreRequest("Floyd")
		response := httptest.NewRecorder()

		server.ServeHTTP(response, request)

		assertStatus(t, response, http.StatusOK)
		assertResponseBody(t, response.Body.String(), "10")
	})

	t.Run("retorna 404 on missing players", func(t *testing.T) {
		request := newGetScoreRequest("Apollo")
		response := httptest.NewRecorder()

		server.ServeHTTP(response, request)

		assertStatus(t, response, http.StatusNotFound)
	})
}

func TestStoreWins(t *testing.T) {
	armazenamento := poquer.EsbocoDeArmazenamentoJogador{
		Pontuações: map[string]int{},
	}
	server := mustMakePlayerServer(t, &armazenamento, dummyGame)

	t.Run("it records venceu on POST", func(t *testing.T) {
		player := "Pepper"

		request := newPostWinRequest(player)
		response := httptest.NewRecorder()

		server.ServeHTTP(response, request)

		assertStatus(t, response, http.StatusAccepted)
		poquer.VerificaVitoriaDoVencedor(t, &armazenamento, player)
	})
}

func TestLeague(t *testing.T) {

	t.Run("it retorna the Liga table as JSON", func(t *testing.T) {
		wantedLeague := []poquer.Jogador{
			{Nome: "Cleo", Vitorias: 32},
			{Nome: "Chris", Vitorias: 20},
			{Nome: "Tiest", Vitorias: 14},
		}

		armazenamento := poquer.EsbocoDeArmazenamentoJogador{Liga: wantedLeague}
		server := mustMakePlayerServer(t, &armazenamento, dummyGame)

		request := newLeagueRequest()
		response := httptest.NewRecorder()

		server.ServeHTTP(response, request)

		obtido := getLeagueFromResponse(t, response.Body)

		assertStatus(t, response, http.StatusOK)
		verificaLiga(t, obtido, wantedLeague)
		assertContentType(t, response, "application/json")

	})
}

func TestGame(t *testing.T) {
	t.Run("GET /partida retorna 200", func(t *testing.T) {
		server := mustMakePlayerServer(t, &poquer.EsbocoDeArmazenamentoJogador{}, dummyGame)

		request := newGameRequest()
		response := httptest.NewRecorder()

		server.ServeHTTP(response, request)

		assertStatus(t, response, http.StatusOK)
	})

	t.Run("start a partida with 3 players, send some blind alerts down WS and declare Ruth the vencedor", func(t *testing.T) {
		wantedBlindAlert := "Blind is 100"
		vencedor := "Ruth"

		partida := &JogoEspiao{AlertaDeBlind: []byte(wantedBlindAlert)}
		server := httptest.NewServer(mustMakePlayerServer(t, ArmazenamentoJogadorTosco, partida))
		ws := mustDialWS(t, "ws"+strings.TrimPrefix(server.URL, "http")+"/ws")

		defer server.Close()
		defer ws.Close()

		writeWSMessage(t, ws, "3")
		writeWSMessage(t, ws, vencedor)

		verificaJogoComeçadoCom(t, partida, 3)
		verificaTerminosChamadosCom(t, partida, vencedor)
		within(t, tenMS, func() { assertWebsocketGotMsg(t, ws, wantedBlindAlert) })
	})
}

func assertWebsocketGotMsg(t *testing.T, ws *websocket.Conn, esperado string) {
	_, msg, _ := ws.ReadMessage()
	if string(msg) != esperado {
		t.Errorf(`obtido "%s", esperado "%s"`, string(msg), esperado)
	}
}

func tentarNovamenteAte(d time.Duration, f func() bool) bool {
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		if f() {
			return true
		}
	}
	return false
}

func within(t *testing.T, d time.Duration, assert func()) {
	t.Helper()

	done := make(chan struct{}, 1)

	go func() {
		assert()
		done <- struct{}{}
	}()

	select {
	case <-time.After(d):
		t.Error("timed out")
	case <-done:
	}
}

func writeWSMessage(t *testing.T, conn *websocket.Conn, message string) {
	t.Helper()
	if err := conn.WriteMessage(websocket.TextMessage, []byte(message)); err != nil {
		t.Fatalf("could not send message over ws connection %v", err)
	}
}

func assertContentType(t *testing.T, response *httptest.ResponseRecorder, esperado string) {
	t.Helper()
	if response.Header().Get("content-type") != esperado {
		t.Errorf("response did not have content-type of %s, obtido %v", esperado, response.HeaderMap)
	}
}

func getLeagueFromResponse(t *testing.T, body io.Reader) []poquer.Jogador {
	t.Helper()
	league, err := poquer.NewLeague(body)

	if err != nil {
		t.Fatalf("Unable to parse response from server '%s' into slice of Jogador, '%v'", body, err)
	}

	return league
}

func verificaLiga(t *testing.T, obtido, esperado []poquer.Jogador) {
	t.Helper()
	if !reflect.DeepEqual(obtido, esperado) {
		t.Errorf("obtido %v esperado %v", obtido, esperado)
	}
}

func assertStatus(t *testing.T, obtido *httptest.ResponseRecorder, esperado int) {
	t.Helper()
	if obtido.Code != esperado {
		t.Errorf("did not get correct status, obtido %d, esperado %d", obtido.Code, esperado)
	}
}

func newGameRequest() *http.Request {
	req, _ := http.NewRequest(http.MethodGet, "/partida", nil)
	return req
}

func newLeagueRequest() *http.Request {
	req, _ := http.NewRequest(http.MethodGet, "/league", nil)
	return req
}

func newGetScoreRequest(nome string) *http.Request {
	req, _ := http.NewRequest(http.MethodGet, fmt.Sprintf("/players/%s", nome), nil)
	return req
}

func newPostWinRequest(nome string) *http.Request {
	req, _ := http.NewRequest(http.MethodPost, fmt.Sprintf("/players/%s", nome), nil)
	return req
}

func assertResponseBody(t *testing.T, obtido, esperado string) {
	t.Helper()
	if obtido != esperado {
		t.Errorf("response body is wrong, obtido '%s' esperado '%s'", obtido, esperado)
	}
}

func mustDialWS(t *testing.T, url string) *websocket.Conn {
	ws, _, err := websocket.DefaultDialer.Dial(url, nil)

	if err != nil {
		t.Fatalf("could not open a ws connection on %s %v", url, err)
	}

	return ws
}
