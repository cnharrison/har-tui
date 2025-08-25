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
		if status >= 400 || status == 0 {
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
		if idx.matchesTextSearch(entry, text) {
			result = append(result, i)
		}
	}
	return result
}

// matchesTextSearch performs comprehensive text matching across all request/response fields
func (idx *EntryIndex) matchesTextSearch(entry HAREntry, searchText string) bool {
	// 1. Search URL (host, path, query parameters)
	if u, err := url.Parse(entry.Request.URL); err == nil {
		if strings.Contains(strings.ToLower(u.Host), searchText) ||
		   strings.Contains(strings.ToLower(u.Path), searchText) ||
		   strings.Contains(strings.ToLower(u.RawQuery), searchText) {
			return true
		}
	}
	
	// 2. Search request method
	if strings.Contains(strings.ToLower(entry.Request.Method), searchText) {
		return true
	}
	
	// 3. Search request headers (names and values)
	for _, header := range entry.Request.Headers {
		if strings.Contains(strings.ToLower(header.Name), searchText) ||
		   strings.Contains(strings.ToLower(header.Value), searchText) {
			return true
		}
	}
	
	// 4. Search response headers (names and values)
	for _, header := range entry.Response.Headers {
		if strings.Contains(strings.ToLower(header.Name), searchText) ||
		   strings.Contains(strings.ToLower(header.Value), searchText) {
			return true
		}
	}
	
	// 5. Search response status text
	if strings.Contains(strings.ToLower(entry.Response.StatusText), searchText) {
		return true
	}
	
	// 6. Search response content type
	if strings.Contains(strings.ToLower(entry.Response.Content.MimeType), searchText) {
		return true
	}
	
	// 7. Search request body (if present and not too large)
	if entry.Request.PostData != nil && entry.Request.PostData.Text != "" {
		// Limit body search to reasonable size for performance
		bodyText := entry.Request.PostData.Text
		if len(bodyText) <= 10000 { // 10KB limit for body search
			if strings.Contains(strings.ToLower(bodyText), searchText) {
				return true
			}
		}
	}
	
	// 8. Search response body (if present and not too large)
	if entry.Response.Content.Text != "" {
		// Decode and search response body
		bodyText := DecodeBase64(entry.Response.Content.Text, entry.Response.Content.Encoding)
		if len(bodyText) <= 10000 { // 10KB limit for body search
			if strings.Contains(strings.ToLower(bodyText), searchText) {
				return true
			}
		}
	}
	
	// 9. Search cookies (names and values)
	for _, cookie := range entry.Request.Cookies {
		if strings.Contains(strings.ToLower(cookie.Name), searchText) ||
		   strings.Contains(strings.ToLower(cookie.Value), searchText) {
			return true
		}
	}
	
	for _, cookie := range entry.Response.Cookies {
		if strings.Contains(strings.ToLower(cookie.Name), searchText) ||
		   strings.Contains(strings.ToLower(cookie.Value), searchText) {
			return true
		}
	}
	
	return false
}

// Use util.IntersectIndices for index intersection operations

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
