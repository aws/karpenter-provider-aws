/*
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package mime

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/mail"
	"net/textproto"
	"strings"

	admapi "github.com/awslabs/amazon-eks-ami/nodeadm/api"
)

type ContentType string

const (
	boundary      = "//"
	versionHeader = "MIME-Version: 1.0"

	ContentTypeShellScript ContentType = `text/x-shellscript; charset="us-ascii"`
	ContentTypeNodeConfig  ContentType = "application/" + admapi.GroupName
	ContentTypeMultipart   ContentType = `multipart/mixed; boundary="` + boundary + `"`
)

type Entry struct {
	ContentType ContentType
	Content     string
}

type Archive []Entry

func NewArchive(content string) (Archive, error) {
	archive := Archive{}
	if content == "" {
		return archive, nil
	}
	reader, err := archive.getReader(content)
	if err != nil {
		return nil, err
	}
	for {
		p, err := reader.NextPart()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, fmt.Errorf("parsing content, %w", err)
		}
		slurp, err := io.ReadAll(p)
		if err != nil {
			return nil, fmt.Errorf("parsing content, %s, %w", string(slurp), err)
		}
		archive = append(archive, Entry{
			ContentType: ContentType(p.Header.Get("Content-Type")),
			Content:     string(slurp),
		})
	}
	return archive, nil
}

// Serialize returns a base64 encoded serialized MIME multi-part archive
func (ma Archive) Serialize() (string, error) {
	buffer := bytes.Buffer{}
	writer := multipart.NewWriter(&buffer)
	if err := writer.SetBoundary(boundary); err != nil {
		return "", err
	}
	buffer.WriteString(versionHeader + "\n")
	buffer.WriteString(fmt.Sprintf("Content-Type: %s\n\n", ContentTypeMultipart))
	for _, entry := range ma {
		partWriter, err := writer.CreatePart(textproto.MIMEHeader{
			"Content-Type": []string{string(entry.ContentType)},
		})
		if err != nil {
			return "", fmt.Errorf("creating multi-part section for entry, %w", err)
		}
		_, err = partWriter.Write([]byte(entry.Content))
		if err != nil {
			return "", fmt.Errorf("writing entry, %w", err)
		}
	}
	if err := writer.Close(); err != nil {
		return "", fmt.Errorf("terminating multi-part archive, %w", err)
	}
	// The mime/multipart package adds carriage returns, while the rest of our logic does not. Remove all
	// carriage returns for consistency.
	return base64.StdEncoding.EncodeToString([]byte(strings.ReplaceAll(buffer.String(), "\r", ""))), nil
}

func (Archive) getReader(content string) (*multipart.Reader, error) {
	mailMsg, err := mail.ReadMessage(strings.NewReader(content))
	if err != nil {
		return nil, err
	}
	mediaType, params, err := mime.ParseMediaType(mailMsg.Header.Get("Content-Type"))
	if err != nil {
		return nil, fmt.Errorf("archive doesn't have Content-Type header, %w", err)
	}
	if !strings.HasPrefix(mediaType, "multipart/") {
		return nil, fmt.Errorf("archive is not in multipart format, %w", err)
	}
	return multipart.NewReader(mailMsg.Body, params["boundary"]), nil
}
