package har

// HARHeader represents an HTTP header in a HAR file
type HARHeader struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// HARCookie represents an HTTP cookie in a HAR file
type HARCookie struct {
	Name     string `json:"name"`
	Value    string `json:"value"`
	Domain   string `json:"domain"`
	Path     string `json:"path"`
	Secure   bool   `json:"secure"`
	HTTPOnly bool   `json:"httpOnly"`
}

// HARPostData represents POST data in a HAR file
type HARPostData struct {
	MimeType string `json:"mimeType"`
	Text     string `json:"text"`
}

// HARRequest represents an HTTP request in a HAR file
type HARRequest struct {
	Method      string        `json:"method"`
	URL         string        `json:"url"`
	HTTPVersion string        `json:"httpVersion"`
	Headers     []HARHeader   `json:"headers"`
	Cookies     []HARCookie   `json:"cookies"`
	PostData    *HARPostData  `json:"postData"`
}

// HARContent represents response content in a HAR file
type HARContent struct {
	Size     int    `json:"size"`
	MimeType string `json:"mimeType"`
	Text     string `json:"text"`
	Encoding string `json:"encoding"`
}

// HARResponse represents an HTTP response in a HAR file
type HARResponse struct {
	Status      int         `json:"status"`
	StatusText  string      `json:"statusText"`
	HTTPVersion string      `json:"httpVersion"`
	Headers     []HARHeader `json:"headers"`
	Cookies     []HARCookie `json:"cookies"`
	Content     HARContent  `json:"content"`
}

// HARTimings represents timing information in a HAR file
type HARTimings struct {
	Blocked float64 `json:"blocked"`
	DNS     float64 `json:"dns"`
	Connect float64 `json:"connect"`
	Send    float64 `json:"send"`
	Wait    float64 `json:"wait"`
	Receive float64 `json:"receive"`
	SSL     float64 `json:"ssl"`
}

// HAREntry represents a single HTTP transaction in a HAR file
type HAREntry struct {
	StartedDateTime string      `json:"startedDateTime"`
	Time            float64     `json:"time"`
	Request         HARRequest  `json:"request"`
	Response        HARResponse `json:"response"`
	Timings         HARTimings  `json:"timings"`
}

// HARLog represents the log object in a HAR file
type HARLog struct {
	Version string     `json:"version"`
	Entries []HAREntry `json:"entries"`
}

// HARFile represents the root HAR file structure
type HARFile struct {
	Log HARLog `json:"log"`
}