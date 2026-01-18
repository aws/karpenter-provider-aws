/*

Package goroutine discovers and returns information about either all goroutines
or only the caller's goroutine. Information provided by the Goroutine type
consists of a unique ID, the state, the name of the topmost (most recent)
function in the call stack and the full backtrace. For goroutines other than the
main goroutine (the one with ID 1) the creating function as well as location
(file name and line number) are additionally provided.

*/
package goroutine
