# ptrequality

[![Test](https://github.com/fillmore-labs/ptrequality/actions/workflows/test.yml/badge.svg?branch=main)](https://github.com/fillmore-labs/ptrequality/actions/workflows/test.yml)
[![Go Report Card](https://goreportcard.com/badge/fillmore-labs.com/ptrequality)](https://goreportcard.com/report/fillmore-labs.com/ptrequality)
[![License](https://img.shields.io/github/license/fillmore-labs/ptrequality)](https://www.apache.org/licenses/LICENSE-2.0)

`ptrequality` is a Go linter (static analysis tool) that detects comparisons against the address of newly created values,
such as `ptr == &MyStruct{}` or `ptr == new(MyStruct)`. These comparisons are almost always incorrect, as each
expression creates a unique allocation at runtime, usually yielding false or undefined results.

## Quickstart

Install the linter:

```console
go install github.com/fillmore-labs/ptrequality/cmd/ptrequality@latest
```

Run the linter on your project:

```console
ptrequality ./...
```

## Examples of Problematic Code

Here are examples that `ptrequality` will flag:

### Direct Pointer Comparisons

```go
import (
  "github.com/operator-framework/api/pkg/operators/v1alpha1"
  metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Checking if a operator update strategy matches expected values
func validateUpdateStrategy(spec *v1alpha1.CatalogSourceSpec) {
  expectedTime := 30 * time.Second

  // ❌ This comparison will always be false - &metav1.Duration{} creates a unique address.
  if (spec.UpdateStrategy.Interval != &metav1.Duration{Duration: expectedTime}) {
    // ...
  }

  // ✅ Correct approach: Dereference the pointer and compare values (after a nil check).
  if spec.UpdateStrategy.Interval == nil || spec.UpdateStrategy.Interval.Duration != expectedTime {
    // ...
  }
}
```

### Error Handling with `errors.Is`

```go
func connectToDatabase() {
  db, err := dbConnect()
  // ❌ This will always be false - &url.Error{} creates a unique address.
  if errors.Is(err, &url.Error{}) {
    log.Fatal("Cannot connect to DB")
  }

  // ✅ Correct approach:
  var urlErr *url.Error
  if errors.As(err, &urlErr) {
    log.Fatal("Error connecting to DB:", urlErr)
  }
  // ...
}

func unmarshalEvent(msg []byte) {
  var es []cloudevents.Event
  err := json.Unmarshal(msg, &es)
  // ❌ This comparison will always be false:
  if errors.Is(err, &json.UnmarshalTypeError{}) {
    //...
  }

  // ✅ Correct approach:
  var typeErr *json.UnmarshalTypeError
  if errors.As(err, &typeErr) {
    //...
  }
}
```

## Special Cases

### `errors.Is` and Similar Functions

`ptrequality` includes special handling for [`errors.Is`](https://pkg.go.dev/errors#Is) and similar functions to reduce
false positives. The linter suppresses diagnostics when:

- **The error type has an `Unwrap() error` method**, as `errors.Is` traverses the error tree.

<details><summary><b><code>Unwrap() error</code> tree example.</b></summary>

```go
type wrappedError struct{ Cause error }

func (e *wrappedError) Error() string { return "wrapped: " + e.Cause.Error() }
func (e *wrappedError) Unwrap() error { return e.Cause } // This suppresses the diagnostic.

  // No warning for this code:
  if errors.Is(&wrappedError{os.ErrNoDeadline}, os.ErrNoDeadline) { // Valid due to "Unwrap" method.
    // ...
  }
```

</details>

- **The error type has an `Is(error) bool` method**, as custom comparison logic is executed.

<details><summary><b>Custom <code>Is(error) bool</code> method example.</b></summary>

When the static type of an error is just the `error` interface, the analyzer cannot know its dynamic type, so the
diagnostic is also suppressed when the _target_ has an `Is(error) bool` method:

```go
type customError struct{ Code int }

func (i *customError) Error() string { return fmt.Sprintf("custom error %d", i.Code) }

func (i *customError) Is(err error) bool { // This suppresses the diagnostic.
  _, ok := err.(*customError)
  return ok
}

  err = func() error {
    return &customError{100}
  }()

  // No warning for this code:
  if errors.Is(err, &customError{200}) { // Valid due to custom "Is" method.
    // ...
  }
```

</details>

#### Rare False Positives

The applied heuristic can lead to false positives in rare cases. For example, if one error type's `Is` method is
designed to compare against a different error type, `ptrequality` may flag valid code. This pattern is uncommon and
potentially confusing.

<details><summary>This workaround improves clarity and suppresses the linting error.</summary>

```go
type errorA struct{ Code int }

func (e *errorA) Error() string { return fmt.Sprintf("error a %d", e.Code) }

type errorB struct{ Code int }

func (e *errorB) Error() string { return fmt.Sprintf("error b %d", e.Code) }

func (e *errorB) Is(err error) bool {
  if err, ok := err.(*errorA); ok { // errorB knows how to check against errorA.
    return e.Code == err.Code
  }

  return false
}

  err := func() error {
    return &errorB{100}
  }()

  // ❌ This valid code gets flagged:
  if errors.Is(err, &errorA{100}) { // Flagged, but technically correct.
    // ...
  }

  // ✅ Document to clarify intent and assign to an identifier to suppress the warning:
  target := &errorA{100} // errorB's "Is" method should match.
  if errors.Is(err, target) {
    // ...
  }
```

</details>

## Diagnostics

- **“Result of comparison with address of new variable of type "..." is always false”**

  This indicates a comparison like `ptr == &MyStruct{}` that will never be true. Consider these fixes:

  - _Compare values instead:_

    ```go
      *ptr == MyStruct{}
    ```

  - _Use `errors.As` for errors:_

    ```go
      var target *MyError
      if errors.As(err, &target) {
        // ...
      }
    ```

  - _Check dynamic type (for interface types):_

    ```go
      if v, ok := v.(*MyStruct); ok {
        if v.SomeField == expected { /* ... */ }
      }
    ```

  - _Pre-declare the target:_

    ```go
      var sentinel = &MyStruct{}
      // ...
      if ptr == sentinel { /* ... */ }
    ```

- **“Result of comparison with address of new variable of type "..." is false or undefined”**

  This diagnostic appears for zero-sized types where the comparison behavior is undefined:

  ```go
  type Skip struct{}

  func (e *Skip) Error() string { return "host hook execution skipped." }

  func (r renderRunner) RunHostHook(ctx context.Context, hook *hostHook) {
    if err := hook.run(ctx /*, ... */); errors.Is(err, &Skip{}) { // ❌ Undefined behavior.
      // ...
    }
  }
  ```

  or

  ```go
      defer func() {
        err := recover()

        if err, ok := err.(error); ok &&
          errors.Is(err, &runtime.PanicNilError{}) { // ❌ Undefined behavior.
          log.Print("panic called with nil argument")
        }
      }()

      panic(nil)
  ```

  While this might work due to Go runtime optimizations, it's logic is unsound. Use `errors.As` instead:

  ```go
    var panicErr *runtime.PanicNilError
    if errors.As(err, &panicErr) {
      log.Println("panic error")
    }
  ```

  For more details, see the blog post
  [_"Equality of Pointers to Zero-Sized Types"_](https://blog.fillmore-labs.com/posts/zerosized-1/).

## License

This project is licensed under the Apache License 2.0. See the [LICENSE](LICENSE) file for details.
