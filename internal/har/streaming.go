package har

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strings"
	"sync"
)

type EntryIndex struct {
	byMethod map[string][]int
	byStatus map[int][]int
	byMime   map[string][]int
	byHost   map[string][]int
	byPath   map[string][]int
	byType   map[string][]int
	mutex    sync.RWMutex
}

func NewEntryIndex() *EntryIndex {
	return &EntryIndex{
		byMethod: make(map[string][]int),
		byStatus: make(map[int][]int),
		byMime:   make(map[string][]int),
		byHost:   make(map[string][]int),
		byPath:   make(map[string][]int),
		byType:   make(map[string][]int),
	}
}

func (idx *EntryIndex) AddEntry(entry HAREntry, index int) {
	idx.mutex.Lock()
	defer idx.mutex.Unlock()

	idx.byMethod[entry.Request.Method] = append(idx.byMethod[entry.Request.Method], index)
	idx.byStatus[entry.Response.Status] = append(idx.byStatus[entry.Response.Status], index)
	idx.byMime[entry.Response.Content.MimeType] = append(idx.byMime[entry.Response.Content.MimeType], index)
	
	if u, err := url.Parse(entry.Request.URL); err == nil {
		idx.byHost[u.Host] = append(idx.byHost[u.Host], index)
		idx.byPath[u.Path] = append(idx.byPath[u.Path], index)
	}
	
	requestType := GetRequestType(entry)
	idx.byType[requestType] = append(idx.byType[requestType], index)
}

func (idx *EntryIndex) GetByMethod(method string) []int {
	idx.mutex.RLock()
	defer idx.mutex.RUnlock()
	result := make([]int, len(idx.byMethod[method]))
	copy(result, idx.byMethod[method])
	return result
}

func (idx *EntryIndex) GetByStatus(status int) []int {
	idx.mutex.RLock()
	defer idx.mutex.RUnlock()
	result := make([]int, len(idx.byStatus[status]))
	copy(result, idx.byStatus[status])
	return result
}

func (idx *EntryIndex) GetByType(requestType string) []int {
	idx.mutex.RLock()
	defer idx.mutex.RUnlock()
	result := make([]int, len(idx.byType[requestType]))
	copy(result, idx.byType[requestType])
	return result
}

func (idx *EntryIndex) GetByHost(host string) []int {
	idx.mutex.RLock()
	defer idx.mutex.RUnlock()
	result := make([]int, len(idx.byHost[host]))
	copy(result, idx.byHost[host])
	return result
}

func (idx *EntryIndex) GetErrorIndices(entries []HAREntry) []int {
	idx.mutex.RLock()
	defer idx.mutex.RUnlock()
	
	var result []int
	for status, indices := range idx.byStatus {
		if status >= 400 {
			result = append(result, indices...)
		}
	}
	return result
}

func (idx *EntryIndex) FilterByText(entries []HAREntry, text string) []int {
	if text == "" {
		result := make([]int, len(entries))
		for i := range entries {
			result[i] = i
		}
		return result
	}
	
	var result []int
	text = strings.ToLower(text)
	
	for i, entry := range entries {
		if u, err := url.Parse(entry.Request.URL); err == nil {
			if strings.Contains(strings.ToLower(u.Host), text) ||
			   strings.Contains(strings.ToLower(u.Path), text) {
				result = append(result, i)
			}
		}
	}
	return result
}

func intersectIndices(a, b []int) []int {
	setA := make(map[int]bool)
	for _, v := range a {
		setA[v] = true
	}
	
	var result []int
	for _, v := range b {
		if setA[v] {
			result = append(result, v)
		}
	}
	return result
}

type StreamingLoader struct {
	entries []HAREntry
	index   *EntryIndex
	mutex   sync.RWMutex
	
	onEntryAdded func(entry HAREntry, index int)
	onComplete   func()
	onError      func(error)
	onProgress   func(count int)
}

func NewStreamingLoader() *StreamingLoader {
	return &StreamingLoader{
		entries: make([]HAREntry, 0),
		index:   NewEntryIndex(),
	}
}

func (sl *StreamingLoader) SetCallbacks(onEntryAdded func(HAREntry, int), onComplete func(), onError func(error), onProgress func(int)) {
	sl.onEntryAdded = onEntryAdded
	sl.onComplete = onComplete
	sl.onError = onError
	sl.onProgress = onProgress
}

func (sl *StreamingLoader) GetEntries() []HAREntry {
	sl.mutex.RLock()
	defer sl.mutex.RUnlock()
	return sl.entries
}

func (sl *StreamingLoader) GetIndex() *EntryIndex {
	return sl.index
}

func (sl *StreamingLoader) GetEntryCount() int {
	sl.mutex.RLock()
	defer sl.mutex.RUnlock()
	return len(sl.entries)
}

func (sl *StreamingLoader) LoadHARFileStreaming(filePath string) {
	go func() {
		file, err := os.Open(filePath)
		if err != nil {
			if sl.onError != nil {
				sl.onError(err)
			}
			return
		}
		defer file.Close()

		decoder := json.NewDecoder(file)
		
		token, err := decoder.Token()
		if err != nil {
			if sl.onError != nil {
				sl.onError(err)
			}
			return
		}
		
		if delim, ok := token.(json.Delim); !ok || delim != '{' {
			if sl.onError != nil {
				sl.onError(fmt.Errorf("expected opening brace"))
			}
			return
		}

		for decoder.More() {
			token, err := decoder.Token()
			if err != nil {
				if sl.onError != nil {
					sl.onError(err)
				}
				return
			}
			
			if key, ok := token.(string); ok && key == "log" {
				if err := sl.parseLog(decoder); err != nil {
					if sl.onError != nil {
						sl.onError(err)
					}
					return
				}
			} else {
				var dummy interface{}
				if err := decoder.Decode(&dummy); err != nil {
					if sl.onError != nil {
						sl.onError(err)
					}
					return
				}
			}
		}

		if sl.onComplete != nil {
			sl.onComplete()
		}
	}()
}

func (sl *StreamingLoader) parseLog(decoder *json.Decoder) error {
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	
	if delim, ok := token.(json.Delim); !ok || delim != '{' {
		return fmt.Errorf("expected opening brace for log object")
	}

	for decoder.More() {
		token, err := decoder.Token()
		if err != nil {
			return err
		}
		
		if key, ok := token.(string); ok && key == "entries" {
			if err := sl.parseEntries(decoder); err != nil {
				return err
			}
		} else {
			var dummy interface{}
			if err := decoder.Decode(&dummy); err != nil {
				return err
			}
		}
	}

	return nil
}

func (sl *StreamingLoader) parseEntries(decoder *json.Decoder) error {
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	
	if delim, ok := token.(json.Delim); !ok || delim != '[' {
		return fmt.Errorf("expected opening bracket for entries array")
	}

	entryCount := 0
	batchSize := 50
	
	for decoder.More() {
		var entry HAREntry
		if err := decoder.Decode(&entry); err != nil {
			return err
		}
		
		sl.mutex.Lock()
		sl.entries = append(sl.entries, entry)
		index := len(sl.entries) - 1
		sl.index.AddEntry(entry, index)
		sl.mutex.Unlock()
		
		if sl.onEntryAdded != nil {
			sl.onEntryAdded(entry, index)
		}
		
		entryCount++
		
		if entryCount%batchSize == 0 && sl.onProgress != nil {
			sl.onProgress(entryCount)
		}
	}

	if sl.onProgress != nil {
		sl.onProgress(entryCount)
	}

	return nil
}
