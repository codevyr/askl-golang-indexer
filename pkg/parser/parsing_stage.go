package parser

import (
	"errors"
	"fmt"
	"runtime"
	"sync"

	"github.com/planetA/askl-golang-indexer/pkg/logging"
)

type ParsingStage struct {
	StageName        string
	StageConstructor ParserConstructor
	parsedPackaged   map[string]bool
	parser           *Parser
	wg               sync.WaitGroup
	errMu            sync.Mutex
	errs             []error
	channel          chan Parsable
	done             chan struct{} // signals loop() has exited
}

func NewParsingStage(parser *Parser, name string, constructor ParserConstructor) *ParsingStage {
	numWorkers := runtime.GOMAXPROCS(0)

	s := &ParsingStage{
		StageName:        name,
		StageConstructor: constructor,
		parsedPackaged:   make(map[string]bool),
		parser:           parser,
		wg:               sync.WaitGroup{},
		channel:          make(chan Parsable, 1000),
		done:             make(chan struct{}),
	}

	go s.loop(numWorkers)
	return s
}

// Parse enqueues an item for processing. The send is non-blocking when the
// channel has capacity; under backpressure a short-lived goroutine is spawned
// to avoid deadlocking when workers recursively submit items.
func (s *ParsingStage) Parse(item Parsable) error {
	s.wg.Add(1)
	select {
	case s.channel <- item:
	default:
		go func() { s.channel <- item }()
	}
	return nil
}

// loop dispatches items from the channel. Duplicate packages are filtered out
// (single-threaded access to parsedPackaged). Non-duplicate items are handed to
// a bounded pool of worker goroutines.
func (s *ParsingStage) loop(numWorkers int) {
	defer close(s.done)

	workerCh := make(chan Parsable, numWorkers)

	var workerWg sync.WaitGroup
	for range numWorkers {
		workerWg.Add(1)
		go func() {
			defer workerWg.Done()
			for item := range workerCh {
				err := item.Parse(s)
				if err != nil {
					logging.Errorf("Error parsing item: %v", err)
					s.errMu.Lock()
					s.errs = append(s.errs, fmt.Errorf("failed to parse: %w", err))
					s.errMu.Unlock()
				}
				s.wg.Done()
			}
		}()
	}

	for item := range s.channel {
		if id, ok := item.GetId(); ok {
			if s.parsedPackaged[id] {
				s.wg.Done()
				continue
			}
			s.parsedPackaged[id] = true
		}
		workerCh <- item
	}

	close(workerCh)
	workerWg.Wait()
}

func (s *ParsingStage) Wait() error {
	s.wg.Wait()

	s.errMu.Lock()
	defer s.errMu.Unlock()

	if len(s.errs) == 0 {
		return nil
	}
	err := fmt.Errorf("parsing stage %s failed: %w", s.StageName, errors.Join(s.errs...))
	s.errs = nil
	return err
}

func (s *ParsingStage) Close() error {
	close(s.channel)
	<-s.done // wait for loop and workers to drain
	return nil
}
