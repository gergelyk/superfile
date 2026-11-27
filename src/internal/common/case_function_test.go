package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConvertCase(t *testing.T) {
	for _, in := range []string{"my file-Name", "myFileName", "MY_FILE_NAME", "my.file.name"} {
		assert.Equal(t, "my_file_name", ConvertCase(in, SnakeCase), in)
		assert.Equal(t, "MyFileName", ConvertCase(in, CamelCase), in)
		assert.Equal(t, "MY_FILE_NAME", ConvertCase(in, UpperCase), in)
		assert.Equal(t, "my-file-name", ConvertCase(in, DashedCase), in)
	}
	assert.Equal(t, "http_server2", ConvertCase("HTTPServer2", SnakeCase))
	assert.Equal(t, "---", ConvertCase("---", SnakeCase))
}
