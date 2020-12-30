package tests

import (
	"testing"

	. "github.com/datastax/burnell/src/route"
)

func TestSubjectMatch(t *testing.T) {
	assert(t, VerifySubject("chris-datastax", "chris-datastax-12345qbc"), "")
	assert(t, VerifySubject("chris-datastax", "chris-datastax-client-12345qbc"), "")
	assert(t, VerifySubject("chris-datastax-client", "chris-datastax-client-client-12345qbc"), "")
	assert(t, VerifySubject("chris-datastax-client", "chris-datastax-client-admin-12345qbc"), "")
	assert(t, VerifySubject("chris-datastax", "chris-datastax-admin-12345qbc"), "")
	assert(t, VerifySubject("your-framework-dev", "your-framework-dev-admin-8e5f5b7412345"), "")
	assert(t, !VerifySubject("your-framework-dev", "your-framework-dev-adMin-8e5f5b7412345"), "")

	assert(t, !VerifySubject("chris-datastax", "chris-datastax"), "")
	assert(t, !VerifySubject("chris-datastax", "chris-datastax-client-client-12345qbc"), "")
	assert(t, !VerifySubject("chris-datastax-client", "chris-datastax-client-client-client-12345qbc"), "")
	assert(t, !VerifySubject("chris-kafkaesque", "chris-datastax-12345qbc"), "")

	t1, t2 := ExtractTenant("chris-datastax-12345qbc")
	equals(t, t1, t2)

	t1, t2 = ExtractTenant("adminuser")
	equals(t, t1, t2)
	equals(t, t1, "adminuser")

	t1, t2 = ExtractTenant("chris-datastax-client-12345qbc")
	equals(t, t1, "chris-datastax-client")
	equals(t, t2, "chris-datastax")

	t1, t2 = ExtractTenant("your-framework-dev-admin-8e5f5b7412345")
	equals(t, t1, "your-framework-dev-admin")
	equals(t, t2, "your-framework-dev")

	t1, t2 = ExtractTenant("chris-datastax-client-client-12345qbc")
	equals(t, t1, "chris-datastax-client-client")
	equals(t, t2, "chris-datastax-client")

	t1, t2 = ExtractTenant("chris-datastax-clien-12345qbc")
	equals(t, t1, "chris-datastax-clien")
	equals(t, t1, t2)

}
