/* Copyright (c) 2021 Bram Vandenbogaerde And Contributors
 * You may use, distribute or modify this code under the
 * terms of the Mozilla Public License 2.0, which is distributed
 * along with the source code.
 */

package scp

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
)

type ResponseType = byte

const (
	Ok      ResponseType = 0
	Warning ResponseType = 1
	Error   ResponseType = 2
	Create  ResponseType = 'C'
	Time    ResponseType = 'T'
)

// ParseResponse reads from the given reader (assuming it is the output of the remote) and parses it into a Response structure.
func ParseResponse(reader io.Reader, writer io.Writer) (*FileInfos, error) {
	fileInfos := NewFileInfos()

	buffer := make([]uint8, 1)
	_, err := reader.Read(buffer)
	if err != nil {
		return fileInfos, err
	}

	responseType := buffer[0]
	message := ""
	if responseType > 0 {
		bufferedReader := bufio.NewReader(reader)
		message, err = bufferedReader.ReadString('\n')
		if err != nil {
			return fileInfos, err
		}

		if responseType == Warning || responseType == Error {
			return fileInfos, errors.New(message)
		}

		// Exit early because we're only interested in the ok response
		if responseType == Ok {
			return fileInfos, nil
		}

		if !(responseType == Create || responseType == Time) {
			return fileInfos, errors.New(
				fmt.Sprintf(
					"Message does not follow scp protocol: %s\n Cmmmm <length> <filename> or T<mtime> 0 <atime> 0",
					message,
				),
			)
		}

		if responseType == Time {
			err = ParseFileTime(message, fileInfos)
			if err != nil {
				return nil, err
			}

			// A custom ssh server can send both time, permissions and size information at once
			// without needing an Ack response. Example: wish from charmbracelet while using their default scp implementation
			// If the buffer is empty, then it's likely the default implementation for ssh, so send Ack
			if bufferedReader.Buffered() == 0 {
				err = Ack(writer)
				if err != nil {
					return fileInfos, err
				}
			}

			message, err = bufferedReader.ReadString('\n')

			if err != nil {
				return fileInfos, err
			}

			responseType = message[0]
		}

		if responseType == Create {
			err = ParseFileInfos(message, fileInfos)
			if err != nil {
				return nil, err
			}
		}
	}

	return fileInfos, nil
}

type FileInfos struct {
	Message     string
	Filename    string
	Permissions uint32
	Size        int64
	Atime       int64
	Mtime       int64
}

func NewFileInfos() *FileInfos {
	return &FileInfos{}
}

func (fileInfos *FileInfos) Update(new *FileInfos) {
	if new == nil {
		return
	}
	if new.Filename != "" {
		fileInfos.Filename = new.Filename
	}
	if new.Permissions != 0 {
		fileInfos.Permissions = new.Permissions
	}
	if new.Size != 0 {
		fileInfos.Size = new.Size
	}
	if new.Atime != 0 {
		fileInfos.Atime = new.Atime
	}
	if new.Mtime != 0 {
		fileInfos.Mtime = new.Mtime
	}
}

func ParseFileInfos(message string, fileInfos *FileInfos) error {
	processMessage := strings.ReplaceAll(message, "\n", "")
	parts := strings.Split(processMessage, " ")
	if len(parts) < 3 {
		return errors.New("unable to parse Chmod protocol")
	}

	permissions, err := strconv.ParseUint(parts[0][1:], 0, 32)
	if err != nil {
		return err
	}

	size, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return err
	}

	fileInfos.Update(&FileInfos{
		Filename:    parts[2],
		Permissions: uint32(permissions),
		Size:        int64(size),
	})

	return nil
}

func ParseFileTime(
	message string,
	fileInfos *FileInfos,
) error {
	processMessage := strings.ReplaceAll(message, "\n", "")
	parts := strings.Split(processMessage, " ")
	if len(parts) < 3 {
		return errors.New("unable to parse Time protocol")
	}

	if len(parts[0]) != 10 {
		return errors.New("length of ATime is not 10")
	}
	mTime, err := strconv.Atoi(parts[0][0:10])
	if err != nil {
		return errors.New("unable to parse ATime component of message")
	}

	if len(parts[2]) != 10 {
		return errors.New("length of MTime is not 10")
	}
	aTime, err := strconv.Atoi(parts[2][0:10])
	if err != nil {
		return errors.New("unable to parse MTime component of message")
	}

	fileInfos.Update(&FileInfos{
		Atime: int64(aTime),
		Mtime: int64(mTime),
	})
	return nil
}

// Ack writes an `Ack` message to the remote, does not await its response, a seperate call to ParseResponse is
// therefore required to check if the acknowledgement succeeded.
func Ack(writer io.Writer) error {
	var msg = []byte{0}
	n, err := writer.Write(msg)
	if err != nil {
		return err
	}
	if n < len(msg) {
		return errors.New("failed to write ack buffer")
	}
	return nil
}
