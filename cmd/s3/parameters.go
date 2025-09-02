package s3

import "kevwargo/ec2-playground/internal/session"

type parameters struct {
	session  *session.Regional
	filename string
	bucket   string
}

func (c parameters) withFilename(filename string) parameters {
	c.filename = filename
	return c
}

func (c parameters) withSession(session *session.Regional) parameters {
	c.session = session
	return c
}
