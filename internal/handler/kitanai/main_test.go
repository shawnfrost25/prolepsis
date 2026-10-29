package kitanai

import (
	"prolepsis/internal/testenv"
	"testing"
)

// Reason why we just don't call 'testenv.Main(m)' inside our TestGetUserById
// First of all, we don't have 'testing.M', and also, 'TestMain' is a special name:
// -- As soon as we run a test, golang will search for TestMain and execute everything it says, so we rather have "TestMain" at package level:
// -- -- "Why?" - because once you run "go test ./internal/handler/kitanai", there won't be TestMain and it will get skipped (leaving you with the package-level variables that 'New' returns to be empty // causing pain)
// -- Since we don't want this to happen, we clearly point inside 'kitanai' that we expect the test to execute 'TestMain' first, which will execute 'Main' (that will execute 'start' and give values to the package-level variables)
func TestMain(m *testing.M) {
	testenv.Main(m)
}
