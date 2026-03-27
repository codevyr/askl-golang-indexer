package parser

import (
	"errors"
	"fmt"
	"sync"

	"github.com/planetA/askl-golang-indexer/pkg/logging"
)

type ParsingStage struct {
	StageName        string
	StageConstructor ParserConstructor
	parsedPackaged   map[string]bool
	parser           *Parser
	wg               sync.WaitGroup
	err              chan error
	channel          chan Parsable
}

func NewParsingStage(parser *Parser, name string, constructor ParserConstructor) *ParsingStage {
	s := &ParsingStage{
		StageName:        name,
		StageConstructor: constructor,
		parsedPackaged:   make(map[string]bool),
		parser:           parser,
		wg:               sync.WaitGroup{},
		err:              make(chan error, 1),
		channel:          make(chan Parsable, 1000),
	}

	go s.loop()
	return s
}

func (s *ParsingStage) Parse(item Parsable) error {

	s.wg.Add(1)
	go func() { s.channel <- item }()

	return nil
}

func (s *ParsingStage) loop() {
	for item := range s.channel {
		err := s.doParse(item)
		if err != nil {
			// Feed the error through the err channel so Wait() can collect it,
			// rather than crashing the process with Fatalf.
			// The error channel triggers wg.Done() in Wait(), balancing the
			// wg.Add(1) from Parse() since doParse didn't launch a goroutine.
			s.err <- fmt.Errorf("failed to parse package: %w", err)
		}
	}
}

func (s *ParsingStage) doParse(item Parsable) error {

	if id, ok := item.GetId(); ok {
		if _, ok := s.parsedPackaged[id]; ok {
			s.wg.Done()

			return nil
		}

		s.parsedPackaged[id] = true
	}

	go func() {
		err := item.Parse(s)
		if err != nil {
			// Send the error to the channel
			logging.Errorf("Error parsing item: %v", err)
			s.err <- fmt.Errorf("failed to parse: %w", err)
		} else {
			s.err <- nil
		}
	}()

	return nil
}

func (s *ParsingStage) Wait() error {
	waitCh := make(chan struct{})
	go func() {
		defer close(waitCh)
		s.wg.Wait()
	}()

	var err error
	for {
		select {
		case <-waitCh:
			if err != nil {
				return fmt.Errorf("parsing stage %s failed: %w", s.StageName, err)
			}
			return nil
		case newErr := <-s.err:
			s.wg.Done()
			if newErr != nil {
				err = errors.Join(err, newErr)
			}
		}
	}
}

func (s *ParsingStage) Close() error {
	close(s.err)
	close(s.channel)
	return nil
}
