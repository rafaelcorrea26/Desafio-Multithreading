package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/rafaelcorrea26/Desafio-Multithreading/internal/cep"
)

const timeoutPadrao = 1 * time.Second

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "erro: %v\n", err)
		os.Exit(1)
	}
}

// a lógica fica aqui e não no main porque o os.Exit não roda os defer
func run() error {
	timeout := flag.Duration("timeout", timeoutPadrao, "tempo limite total da consulta")
	flag.Usage = usage
	flag.Parse()

	if flag.NArg() != 1 {
		flag.Usage()
		return errors.New("informe exatamente um CEP")
	}

	codigo, err := cep.Normalize(flag.Arg(0))
	if err != nil {
		return err
	}

	// pra adicionar uma terceira API é só incluir aqui
	provedores := []cep.Provider{cep.NewBrasilAPI(), cep.NewViaCEP()}

	fmt.Printf("Consultando o CEP %s em %d APIs simultaneamente (timeout de %s)...\n\n",
		cep.Format(codigo), len(provedores), *timeout)

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	resultado, err := cep.Race(ctx, codigo, provedores...)
	if err != nil {
		switch {
		case errors.Is(err, cep.ErrTimeout):
			return fmt.Errorf("timeout: nenhuma API respondeu em %s", *timeout)
		case errors.Is(err, cep.ErrCEPNaoEncontrado):
			return fmt.Errorf("CEP %s não encontrado", cep.Format(codigo))
		default:
			return err
		}
	}

	fmt.Printf("API vencedora: %s (respondeu em %s)\n", resultado.Provider, resultado.Elapsed.Round(time.Millisecond))
	fmt.Println("-----------------------------------")
	fmt.Println(resultado.Address)
	return nil
}

func usage() {
	fmt.Fprintf(os.Stderr, "Uso: %s [flags] <cep>\n\nExemplos:\n  %s 01153000\n  %s -timeout=2s 01153-000\n\nFlags:\n",
		os.Args[0], os.Args[0], os.Args[0])
	flag.PrintDefaults()
}
