package main

import (
	"fmt"
)

type coords struct {
	x, y int
}

func (c coords) String() string {
	if c.x < 0 || 7 < c.x || c.y < 0 || 7 < c.y {
		panic(fmt.Sprint("Illegal coordinates (%d, %d)", c.x, c.y))
	}
	return fmt.Sprintf("%c%d", "ABCDEFGH"[c.x], c.y+1)
}

func (c *coords) Scan(state fmt.ScanState, verb rune) error {
	rx, _, _ := state.ReadRune()
	ry, _, _ := state.ReadRune()
	if rx < 'A' || 'G' < rx || ry < '1' || '8' < ry {
		return fmt.Errorf("Illegal chess coordinates: <%c, %c>", rx, ry)
	}
	c.x = int(rx - 'A')
	c.y = int(ry - '1')
	return nil
}

type pieceBareType int

const (
	PAWN pieceBareType = iota
	KNIGHT
	BISHOP
	ROOK
	QUEEN
	KING
)

type pieceColor int

const (
	BLACK pieceColor = iota
	WHITE
)

type pieceType struct {
	t pieceBareType
	c pieceColor
}

func (pt pieceType) String() string {
	switch pt.c {
	case WHITE:
		switch pt.t {
		case PAWN:
			return "♙"
		case KNIGHT:
			return "♘"
		case BISHOP:
			return "♗"
		case ROOK:
			return "♖"
		case QUEEN:
			return "♕"
		case KING:
			return "♔"
		}
		break
	case BLACK:
		switch pt.t {
		case PAWN:
			return "♟"
		case KNIGHT:
			return "♞"
		case BISHOP:
			return "♝"
		case ROOK:
			return "♜"
		case QUEEN:
			return "♛"
		case KING:
			return "♚"
		}
		break
	}
	panic("Illegal piece type")
}

// Operations on pieces
type pop interface{}

type popGetCoords chan<- coords

type popSetCoords coords

type popMove coords

// Die. Close this channel when operation acknowledged (for sync)
type popKill chan<- bool

type popSetType pieceType

type popGetType chan<- pieceType

type piece chan<- pop

func (p piece) ptype() pieceType {
	pc := make(chan pieceType)
	p <- popGetType(pc)
	return <-pc
}

// Returns nil if the attempted move is valid
func (p piece) validate(m popMove) error {
	// TODO
	return nil
}

func (p piece) Move(to coords) error {
	op := popMove(to)
	if err := p.validate(op); err != nil {
		return err
	}
	p <- op
	return nil
}

// Operations on a chess board
type bop interface{}

// Place a new piece on the board
type bopNewPiece struct {
	coords
	p piece
}

type bopMovePiece struct {
	from, to coords
}

type bopGetPiece struct {
	coords
	result chan<- piece
}

type bopGetAllPieces chan<- piece

type bopDelPiece piece

type board chan<- bop

func (b board) Get(loc coords) piece {
	c := make(chan piece)
	b <- bopGetPiece{loc, c}
	return <-c
}

// Control operations are read from the control channel.
func spawnPiece(b board, c <-chan pop) {
	var loc coords
	var pt pieceType
	for op := range c {
		switch t := op.(type) {
		case popMove:
			to := coords(t)
			b <- bopMovePiece{loc, to}
			loc = to
		case popSetCoords:
			loc = coords(t)
		case popGetCoords:
			t <- loc
			close(t)
		case popKill:
			close(t)
			return
		case popSetType:
			pt = pieceType(t)
		case popGetType:
			t <- pt
			close(t)
		default:
			panic(fmt.Sprintf("Illegal operation: %v", op))
		}
	}
}

func addPiece(x, y int, pt pieceType, b board) piece {
	// Start a piece
	c := make(chan pop)
	go spawnPiece(b, c)
	// Make it a pawn
	c <- popSetType(pt)
	// Move it to the desired coordinates
	c <- popSetCoords{x, y}
	b <- bopNewPiece{coords: coords{x, y}, p: c}
	return c
}

func addPawn(x int, color pieceColor, b board) piece {
	var y int
	if color == WHITE {
		y = 1
	} else {
		y = 6
	}
	return addPiece(x, y, pieceType{PAWN, color}, b)
}

func baseline(color pieceColor) int {
	if color == WHITE {
		return 0
	}
	return 7
}

func addKnight(x int, color pieceColor, b board) piece {
	return addPiece(x, baseline(color), pieceType{KNIGHT, color}, b)
}

func addBishop(x int, color pieceColor, b board) piece {
	return addPiece(x, baseline(color), pieceType{BISHOP, color}, b)
}

func addRook(x int, color pieceColor, b board) piece {
	return addPiece(x, baseline(color), pieceType{ROOK, color}, b)
}

func addQueen(color pieceColor, b board) piece {
	return addPiece(3, baseline(color), pieceType{QUEEN, color}, b)
}

func addKing(color pieceColor, b board) piece {
	return addPiece(4, baseline(color), pieceType{KING, color}, b)
}

// Run a board management unit.  Closes the done channel when all updates have
// been consumed and the input channel is closed (for sync).
func runBoard(c <-chan bop, done chan<- bool) {
	pieces := map[coords]piece{}
	for o := range c {
		switch t := o.(type) {
		case bopNewPiece:
			if _, exists := pieces[t.coords]; exists {
				panic(fmt.Sprintf("A piece already exists on %s", t.coords))
			}
			pieces[t.coords] = t.p
			fmt.Printf("New piece: %s on %s\n", t.p.ptype(), t.coords)
			break
		case bopMovePiece:
			p, exists := pieces[t.from]
			if !exists {
				panic(fmt.Sprintf("No piece at %s", t.from))
			}
			pt := p.ptype()
			delete(pieces, t.from)
			pieces[t.to] = p
			fmt.Printf("Move %s from %s to %s\n", pt, t.from, t.to)
		case bopGetPiece:
			if p, ok := pieces[t.coords]; ok {
				t.result <- p
			}
			close(t.result)
		case bopGetAllPieces:
			for _, p := range pieces {
				t <- p
			}
			close(t)
			break
		case bopDelPiece:
			donec := make(chan bool)
			cc := make(chan coords)
			t <- popGetCoords(cc)
			coords := <-cc
			t <- popKill(donec)
			<-donec
			delete(pieces, coords)
			fmt.Printf("Deleted piece from %s\n", coords)
			break
		default:
			panic(fmt.Sprintf("Illegal board operation: %v", o))
		}
	}
	close(done)
}

func initBoard1p(b board, color pieceColor) {
	addPawn(0, color, b)
	addPawn(1, color, b)
	addPawn(2, color, b)
	addPawn(3, color, b)
	addPawn(4, color, b)
	addPawn(5, color, b)
	addPawn(6, color, b)
	addPawn(7, color, b)
	addRook(0, color, b)
	addKnight(1, color, b)
	addBishop(2, color, b)
	addQueen(color, b)
	addKing(color, b)
	addBishop(5, color, b)
	addKnight(6, color, b)
	addRook(7, color, b)
}

// Initialize an empty chess board by putting pieces in the right places
func initBoard(b board) {
	initBoard1p(b, WHITE)
	initBoard1p(b, BLACK)
}

func clearBoard(b board) {
	piecesc := make(chan piece)
	b <- bopGetAllPieces(piecesc)
	// Two-step to avoid dead-lock
	pieces := []piece{}
	for p := range piecesc {
		pieces = append(pieces, p)
	}
	for _, p := range pieces {
		b <- bopDelPiece(p)
	}
	return
}

// Parse human-readable coordinates into a move operation
func parseMoveOp(from, to string) (op bopMovePiece, err error) {
	_, err = fmt.Sscan(from, &op.from)
	if err != nil {
		return
	}
	_, err = fmt.Sscan(to, &op.to)
	return
}

func main() {
	boardc := make(chan bop)
	boarddone := make(chan bool)
	go runBoard(boardc, boarddone)
	initBoard(boardc)
	// Open with a white pawn
	op, err := parseMoveOp("D2", "D4")
	if err != nil {
		panic(err.Error())
	}
	b := board(boardc)
	b.Get(op.from).Move(op.to)
	clearBoard(boardc)
}
