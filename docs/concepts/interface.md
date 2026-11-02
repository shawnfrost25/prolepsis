## Where I Noticed the Gap

I was trying to intercept GitHub's callback function for a test ([CLICK ME](https://github.com/shawnfrost25/prolepsis/blob/main/internal/handler/github/callback_test.go)) when I realized that I had no idea how to intercept a function.

While investigating, I learned that the key to intercepting certain functions is understanding the interfaces behind them and, more importantly, what those interfaces expect in order to be satisfied.

## The Problem

I always thought of interfaces as some kind of closures, or perhaps a globally declared set of functions. However, that interpretation didn't make much sense, especially since ordinary functions can already be declared globally.

I also assumed that interfaces were primarily something used internally by libraries, so I never really bothered to learn them.

That's where I noticed the gap: **I was overlooking one of the fundamental concepts that makes function interception and dependency substitution possible in Go.**

## Underlying Concept / How It Works

I was treating interfaces as some sort of *Function 2.0*, when their actual purpose is entirely different.

An interface defines a **contract**: a set of method signatures that a type must implement to satisfy that interface.

In other words, an interface specifies what a type must be able to do, rather than how it should do it.

This is what makes interception possible.

Instead of depending on one specific implementation, a function can depend on an interface. As long as our replacement implementation satisfies the interface's requirements, we can substitute it for the original implementation.

The correct mental model isn't:

- "Interfaces are just globally declared closures."

It's:

- "An interface defines a contract. Any type that satisfies that contract can be used wherever that interface is expected."

This means we can replace an existing implementation with our own, provided that we follow the same contract.

## Workflow / Lifecycle (Diagram)

//

## Example Code and Explanation

Here, we'll look at how Go's `net/http` package allows us to intercept outgoing HTTP requests through `http.RoundTripper`.

```go
// The interface responsible for handling outgoing HTTP requests.
type RoundTripper interface {
    RoundTrip(*Request) (*Response, error)
}
```

Let's break down what this means.

This interface defines a single method: `RoundTrip`.

It expects:
- A pointer to an `http.Request` as its input.
- A pointer to an `http.Response` as its output.
- An `error` to indicate whether something went wrong.

But how does this interface actually help us intercept requests?

**It's the contract that makes interception possible in the first place.**

The `http.Client` doesn't need to know the exact implementation responsible for sending the request. It only needs something that satisfies the `RoundTripper` interface.

That means we can provide our own implementation of `RoundTripper`, receive the outgoing request, inspect or modify it, and return a response without necessarily making a real network request.

In other words, we can replace the original HTTP transport with our own implementation, as long as it satisfies the interface.

That's the underlying mechanism that allows us to intercept HTTP requests in tests.

## What I Still Don't Understand

//
