package parser

import (
	"log"
	"sync"
)

type ParsingStage struct {
	StageName        string
	StageConstructor ParserConstructor
	parsedPackaged   map[string]bool
	parser           *Parser
	wg               sync.WaitGroup
	channel          chan Parsable
}

func NewParsingStage(parser *Parser, name string, constructor ParserConstructor) *ParsingStage {
	s := &ParsingStage{
		StageName:        name,
		StageConstructor: constructor,
		parsedPackaged:   make(map[string]bool),
		parser:           parser,
		wg:               sync.WaitGroup{},
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
			log.Fatalf("failed to parse package: %v", err)
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
		defer s.wg.Done()
		err := item.Parse(s)
		if err != nil {
			log.Fatalf("failed to parse: %s", err)
		}
	}()

	return nil
}

func (s *ParsingStage) Wait() {
	s.wg.Wait()
}

func (s *ParsingStage) Close() error {
	close(s.channel)
	return nil
}
