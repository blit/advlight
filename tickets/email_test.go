package tickets

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSMTPConfig(t *testing.T) {
	const cstr = "32048230948902358023:Aqo6jcr4r9/Nq59S+Cu4qAIE3uSpfAl:email-smtp.us-east-1.amazonaws.com:2587"
	c := &smtpconfig{}
	err := c.Parse(cstr)
	assert.NoError(t, err)
	assert.Equal(t, "32048230948902358023", c.Username)
	assert.Equal(t, "Aqo6jcr4r9/Nq59S+Cu4qAIE3uSpfAl", c.Password)
	assert.Equal(t, "email-smtp.us-east-1.amazonaws.com", c.Hostname)
	assert.Equal(t, 2587, c.Port)
}
