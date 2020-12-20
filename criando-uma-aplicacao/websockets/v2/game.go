package poquer

import "io"

// Game manages the state of a partida
type Game interface {
	Começar(numeroDeJogadores int, alertsDestination io.Writer)
	Terminar(vencedor string)
}
