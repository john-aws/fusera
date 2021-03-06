// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package nr

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"time"

	"github.com/mattrbianchi/twig"
	"github.com/pkg/errors"
)

func ResolveNames(url, loc string, ngc []byte, accs map[string]bool) (map[string]Accession, error) {
	if url == "" {
		url = "https://www.ncbi.nlm.nih.gov/Traces/names/names.fcgi"
		twig.Debugf("Name Resolver endpoint was empty, using default: %s", url)
	}
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	if ngc != nil {
		// handle ngc bytes
		part, err := writer.CreateFormFile("ngc", "ngc")
		if err != nil {
			return nil, errors.Wrapf(err, "couldn't create form file for ngc")
		}
		_, err = io.Copy(part, bytes.NewReader(ngc))
		if err != nil {
			return nil, errors.Errorf("couldn't copy ngc contents: %s into multipart file to make request", ngc)
		}

	}
	if err := writer.WriteField("version", "xc-1.0"); err != nil {
		return nil, errors.New("could not write version field to multipart.Writer")
	}
	if err := writer.WriteField("format", "json"); err != nil {
		return nil, errors.New("could not write format field to multipart.Writer")
	}
	if loc != "" {
		if err := writer.WriteField("location", loc); err != nil {
			return nil, errors.New("could not write loc field to multipart.Writer")
		}
	}
	if accs != nil {
		for acc, _ := range accs {
			if err := writer.WriteField("acc", acc); err != nil {
				return nil, errors.New("could not write acc field to multipart.Writer")
			}
		}
	}
	twig.Debug("version: xc-1.0")
	twig.Debug("format: json")
	twig.Debugf("location: %s", loc)
	twig.Debugf("acc: %v", accs)
	if err := writer.Close(); err != nil {
		return nil, errors.New("could not close multipart.Writer")
	}

	req, err := http.NewRequest("POST", url, body)
	if err != nil {
		return nil, errors.New("can't create request to Name Resolver API")
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	twig.Debugf("HTTP REQUEST:\n %+v", req)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, errors.New("can't resolve acc names")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, errors.Errorf("encountered error from Name Resolver API: %s", resp.Status)
	}
	ct := resp.Header.Get("Content-Type")
	if ct != "application/json" {
		return nil, errors.Errorf("Name Resolver API gave incorrect Content-Type: %s", ct)
	}

	bytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.New("fatal error when trying to read response from Name Resolver API")
	}
	content := string(bytes)
	twig.Debugf("Response Body from API:\n%s", content)
	var payload []Payload
	err = json.Unmarshal(bytes, &payload)
	if err != nil {
		var errPayload Payload
		err = json.Unmarshal(bytes, &errPayload)
		if err != nil {
			return nil, errors.New("fatal error when trying to read response from Name Resolver API")
		}
		return nil, errors.Errorf("encountered error from Name Resolver API: %d: %s", errPayload.Status, errPayload.Message)
	}

	accessions, msg, err := sanitize(payload)
	if msg != "" && err == nil {
		fmt.Println(msg)
	}

	return accessions, err
}

// msg is used to develop a message to the user indicating which accessions did not succeed while keeping err useful for disastrous errors.
func sanitize(payload []Payload) (accs map[string]Accession, msg string, err error) {
	errmsg := ""
	accs = make(map[string]Accession)
	for _, p := range payload {
		if p.Status != http.StatusOK {
			msg = msg + fmt.Sprintf("issue with accession %s: %s\n", p.ID, p.Message)
			errmsg = errmsg + fmt.Sprintf("%s: %d\t%s", p.ID, p.Status, p.Message)
			continue
		}
		// get existing acc or make a new one
		acc := Accession{ID: p.ID, Files: make(map[string]File)}
		if a, ok := accs[p.ID]; ok {
			// so we have a duplicate acc...
			acc = a
		}
		for _, f := range p.Files {
			if f.Link == "" {
				msg = msg + fmt.Sprintf("issue with accession %s: API returned no link for %s\n", p.ID, f.Name)
				continue
			}
			if f.Name == "" {
				msg = msg + fmt.Sprintf("issue with accession %s: API returned no name for %s\n", p.ID, f)
				continue
			}
			acc.Files[f.Name] = f
		}
		// finally finished with acc
		accs[acc.ID] = acc
	}
	if len(accs) < 1 {
		err = errors.Errorf("API returned no mountable accessions\n%s", errmsg)
	}
	return
}

type Payload struct {
	ID      string `json:"accession,omitempty"`
	Status  int    `json:"status,omitempty"`
	Message string `json:"message,omitempty"`
	Files   []File `json:"files,omitempty"`
}

type Accession struct {
	ID    string `json:"accession,omitempty"`
	Files map[string]File
}

type File struct {
	Name           string    `json:"name,omitempty"`
	Size           string    `json:"size,omitempty"`
	ModifiedDate   time.Time `json:"modificationDate,omitempty"`
	Md5Hash        string    `json:"md5,omitempty"`
	Link           string    `json:"link,omitempty"`
	ExpirationDate time.Time `json:"expirationDate,omitempty"`
	Service        string    `json:"service,omitempty"`
}
