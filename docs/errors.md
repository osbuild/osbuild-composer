# Error Handling

## When to Panic

Always use `panic` for errors that can only happen when other code in
*osbuild-composer* is wrong (also know as *programmer error*). This way, we
catch these kinds of errors in unit tests while developing.

Since only developers interact with these errors, a stacktrace including the
error is all that's necessary. Don't include an additional message.

For example, Go's `json.Marshal` can fail when receiving values that cannot be
marshaled. However, when passing a known struct, we know it cannot fail:

```golang
bytes, err := json.Marshal();
if err != nil {
        panic(err)
}
```

Some packages have functions prefixed with `Must`, which `panic()` on error.
Use these when possible to save the error check:

```golang
re := regexp.MustCompile("v[0-9]")
```
