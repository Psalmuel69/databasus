package ftp_storage

import (
	"errors"
	"net/textproto"
	"testing"

	"github.com/jlaffaye/ftp"
	"github.com/stretchr/testify/assert"
)

func Test_IsFtpFileUnavailable_TellsAMissingFileFromAFailedConnection(t *testing.T) {
	cases := map[string]struct {
		err      error
		isAbsent bool
	}{
		"file unavailable": {
			err:      &textproto.Error{Code: ftp.StatusFileUnavailable, Msg: "No such file or directory"},
			isAbsent: true,
		},
		"transient error": {
			err:      &textproto.Error{Code: ftp.StatusFileActionIgnored, Msg: "File busy"},
			isAbsent: false,
		},
		"command not implemented": {
			err:      &textproto.Error{Code: ftp.StatusCommandNotImplemented, Msg: "DELE not supported"},
			isAbsent: false,
		},
		"connection failure": {
			err:      errors.New("connection reset by peer"),
			isAbsent: false,
		},
	}

	for name, testCase := range cases {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, testCase.isAbsent, isFtpFileUnavailable(testCase.err))
		})
	}
}
