package tests

import (
	"testing"

	. "github.com/kafkaesque-io/burnell/src/route"
)

func TestSubjectMatch(t *testing.T) {
	assert(t, VerifySubject("chris-kafkaesque-io", "chris-kafkaesque-io-12345qbc"), "")
	assert(t, VerifySubject("chris-kafkaesque-io", "chris-kafkaesque-io-client-12345qbc"), "")
	assert(t, VerifySubject("chris-kafkaesque-io-client", "chris-kafkaesque-io-client-client-12345qbc"), "")
	assert(t, VerifySubject("chris-kafkaesque-io-client", "chris-kafkaesque-io-client-admin-12345qbc"), "")
	assert(t, VerifySubject("chris-kafkaesque-io", "chris-kafkaesque-io-admin-12345qbc"), "")
	assert(t, VerifySubject("your-framework-dev", "your-framework-dev-admin-8e5f5b7412345"), "")
	assert(t, !VerifySubject("your-framework-dev", "your-framework-dev-adMin-8e5f5b7412345"), "")

	assert(t, !VerifySubject("chris-kafkaesque-io", "chris-kafkaesque-io"), "")
	assert(t, !VerifySubject("chris-kafkaesque-io", "chris-kafkaesque-io-client-client-12345qbc"), "")
	assert(t, !VerifySubject("chris-kafkaesque-io-client", "chris-kafkaesque-io-client-client-client-12345qbc"), "")
	assert(t, !VerifySubject("chris-kafkaesque", "chris-kafkaesque-io-12345qbc"), "")

	t1, t2 := ExtractTenant("chris-kafkaesque-io-12345qbc")
	equals(t, t1, t2)

	t1, t2 = ExtractTenant("adminuser")
	equals(t, t1, t2)
	equals(t, t1, "adminuser")

	t1, t2 = ExtractTenant("chris-kafkaesque-io-client-12345qbc")
	equals(t, t1, "chris-kafkaesque-io-client")
	equals(t, t2, "chris-kafkaesque-io")

	t1, t2 = ExtractTenant("your-framework-dev-admin-8e5f5b7412345")
	equals(t, t1, "your-framework-dev-admin")
	equals(t, t2, "your-framework-dev")

	t1, t2 = ExtractTenant("chris-kafkaesque-io-client-client-12345qbc")
	equals(t, t1, "chris-kafkaesque-io-client-client")
	equals(t, t2, "chris-kafkaesque-io-client")

	t1, t2 = ExtractTenant("chris-kafkaesque-io-clien-12345qbc")
	equals(t, t1, "chris-kafkaesque-io-clien")
	equals(t, t1, t2)

}
