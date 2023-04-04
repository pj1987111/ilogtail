// Copyright 2021 iLogtail Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package kvsplitter

import (
	"strconv"
	"strings"

	"github.com/alibaba/ilogtail/pkg/logger"
	"github.com/alibaba/ilogtail/pkg/pipeline"
	"github.com/alibaba/ilogtail/pkg/protocol"
)

type KeyValueSplitter struct {
	SourceKey string
	// Split key/value pairs.
	Delimiter string
	// Split key and value.
	Separator            string
	KeepSource           bool
	EmptyKeyPrefix       string
	NoSeparatorKeyPrefix string
	QuoteFlag            bool
	Quote                string

	DiscardWhenSeparatorNotFound bool
	ErrIfSourceKeyNotFound       bool
	ErrIfSeparatorNotFound       bool
	ErrIfKeyIsEmpty              bool

	context pipeline.Context
}

const (
	defaultDelimiter            = "\t"
	defaultSeparator            = ":"
	defaultEmptyKeyPrefix       = "empty_key_"
	defaultNoSeparatorKeyPrefix = "no_separator_key_"
)

func (s *KeyValueSplitter) Init(context pipeline.Context) error {
	s.context = context

	if len(s.Delimiter) == 0 {
		s.Delimiter = defaultDelimiter
	}
	if len(s.Separator) == 0 {
		s.Separator = defaultSeparator
	}
	if len(s.EmptyKeyPrefix) == 0 {
		s.EmptyKeyPrefix = defaultEmptyKeyPrefix
	}
	if len(s.NoSeparatorKeyPrefix) == 0 {
		s.NoSeparatorKeyPrefix = defaultNoSeparatorKeyPrefix
	}
	return nil
}

func (*KeyValueSplitter) Description() string {
	return "Processor to split key value pairs"
}

func (s *KeyValueSplitter) ProcessLogs(logArray []*protocol.Log) []*protocol.Log {
	for _, log := range logArray {
		s.processLog(log)
	}
	return logArray
}

func (s *KeyValueSplitter) processLog(log *protocol.Log) {
	hasKey := false
	for idx, content := range log.Contents {
		if len(s.SourceKey) == 0 || s.SourceKey == content.Key {
			hasKey = true
			if !s.KeepSource {
				log.Contents = append(log.Contents[:idx], log.Contents[idx+1:]...)
			}
			s.splitKeyValue(log, content.Value)
			break
		}
	}
	if !hasKey && s.ErrIfSourceKeyNotFound {
		logger.Warningf(s.context.GetRuntimeContext(), "KV_SPLITTER_ALARM", "can not find key: %v", s.SourceKey)
	}
}

func (s *KeyValueSplitter) splitKeyValue(log *protocol.Log, content string) {
	emptyKeyIndex := 0
	noSeparatorKeyIndex := 0
	dIdx := 0
	for dIdx < len(content) {
		dIdx = strings.Index(content, s.Delimiter)
		var pair string
		if dIdx == -1 {
			pair = content
		} else {
			pair = content[:dIdx]
		}

		pair, dIdx = s.concatQuotePair(pair, content, dIdx)
		pos := strings.Index(pair, s.Separator)
		if pos == -1 {
			if s.ErrIfSeparatorNotFound {
				logger.Warningf(s.context.GetRuntimeContext(), "KV_SPLITTER_ALARM", "can not find separator in %v", pair)
			}
			if !s.DiscardWhenSeparatorNotFound {
				log.Contents = append(log.Contents, &protocol.Log_Content{
					Key:   s.NoSeparatorKeyPrefix + strconv.Itoa(noSeparatorKeyIndex),
					Value: s.getValue(pair),
				})
				noSeparatorKeyIndex++
			}
		} else {
			key := pair[:pos]
			value := s.getValue(pair[pos+len(s.Separator):])
			if len(key) == 0 {
				key = s.EmptyKeyPrefix + strconv.Itoa(emptyKeyIndex)
				emptyKeyIndex++
				if s.ErrIfKeyIsEmpty {
					logger.Warningf(s.context.GetRuntimeContext(), "KV_SPLITTER_ALARM",
						"the key of pair with value (%v) is empty", value)
				}
			}
			log.Contents = append(log.Contents, &protocol.Log_Content{Key: key, Value: value})
		}

		if dIdx == -1 {
			break
		}
		if dIdx+len(s.Delimiter) <= len(content) {
			content = content[dIdx+len(s.Delimiter):]
		}
	}
}

func (s *KeyValueSplitter) concatQuotePair(pair string, content string, dIdx int) (string, int) {
	// If Pair not end with quote,try to reIndex the pair
	if s.QuoteFlag && len(s.Quote) > 0 && !strings.HasSuffix(pair, s.Quote) {
		// ReIndex from last delimiter to find next quote index
		lastQuote := strings.Index(content[dIdx+1:], s.Quote)
		// Separator+Quote or Quote in prefix
		if strings.Index(pair, s.Separator+s.Quote) > 0 || strings.HasPrefix(pair, s.Quote) {
			lastQuote += len(s.Separator + s.Quote)
			if lastQuote > 0 {
				dIdx += lastQuote
				pair = content[:dIdx]
			}
		}
	}
	return pair, dIdx
}

func (s *KeyValueSplitter) getValue(value string) string {
	if s.QuoteFlag && len(s.Quote) > 0 {
		// remove quote
		if len(value) >= 2*len(s.Quote) && strings.HasPrefix(value, s.Quote) && strings.HasSuffix(value, s.Quote) {
			value = value[len(s.Quote) : len(value)-len(s.Quote)]
		}
	}
	return value
}

func newKeyValueSplitter() *KeyValueSplitter {
	return &KeyValueSplitter{
		Delimiter:                    defaultDelimiter,
		Separator:                    defaultSeparator,
		KeepSource:                   true,
		EmptyKeyPrefix:               defaultEmptyKeyPrefix,
		NoSeparatorKeyPrefix:         defaultNoSeparatorKeyPrefix,
		ErrIfSourceKeyNotFound:       true,
		ErrIfSeparatorNotFound:       true,
		ErrIfKeyIsEmpty:              true,
		DiscardWhenSeparatorNotFound: false,
	}
}

func init() {
	pipeline.Processors["processor_split_key_value"] = func() pipeline.Processor {
		return newKeyValueSplitter()
	}
}
