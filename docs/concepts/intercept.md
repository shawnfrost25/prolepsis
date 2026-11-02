## Where I Noticed the Gap
* I was trying to create tests for the GitHub callback function when I noticed a problem: it would consistently fail whenever we reached the part where an HTTP request had to be sent over the network. I initially started creating mock implementations of the functions involved, but they always ended up being problematic. They didn't properly follow the original processes, skipped important steps, and simply acted as if everything had worked successfully. In many cases, this concealed errors and produced false confidence, even when the original function would have failed.

## The Problem
* For example, when attempting to exchange an authorization code for an access token, the function would fail because it expected a real authorization code. Similarly, whenever a secret or other required value was involved, we were forced to jump through hoops just to get the test running. However, by intercepting the function's behavior, we can control how it acts and what it returns without replacing the entire process. This allows us to determine the outcome we need for a test while still letting the original function execute its actual logic. In other words, rather than pretending that the function succeeded, we let it run and control the external behavior it depends on.

## Underlying Concept / How It Works

* **How to work with them:**
    * **How to understand what you are provided with when you are creating the intercept function:**
        * Enter the function you wanna inspect and look for the body and headers they use (simply search for them, watch out to see if they will be in form of an uri or a json body / search for Content-Type) - as example, enter the definition of "Exchange" and there are the fields that it provides us ("code", "redirect_uri", etc...) // Also, look at what you hand do the function
    * **How to perceive what you must send back (the response your function must give):**
        * Simply enter the function you wanna inspect, follow the request it created for us (the one we discussed about above) and watch when it will exchange it to get a response (usually via ".Do()"), then watch what the code does with the response, as what fields it expects and stuff
* **What's happening inside**
* Mental mode: Think of it as a "what you send is what your function will obtain and what will return will pass through a little more processes"

## Workflow / Lifecycle (Diagram)
//

## Example Code and Explanation
I'll show an example of how we can intercept the `*oauth2.Config.Exchange()` function and explain the process in simple terms.

**Step 1 - Find the interface behind it**
The first thing to do is inspect the function. Use *Go to Definition* and follow the code to understand how your request is handled and where it eventually goes.

**Step 2 - Understand the contract**
Once you've reached the relevant interface, inspect the contract we need to satisfy.
In our case, we need to satisfy the `http.RoundTripper` interface, which expects a method that accepts a request as a parameter and returns a response or an error.
```go
type RoundTripper interface {
    RoundTrip(*Request) (*Response, error)
}
```

**Step 3 - Create our own implementation**
Now that we know what the interface expects, we can create our own structure (the name doesn't matter) and implement the required method.
Inside this method, we decide what to do: what to check, how to handle the request, and what response to return.
**However, we must first understand what happens to the request and response throughout the process.** Follow the request as it travels through the functions, inspect what information is provided to us, and finally, examine what the calling code expects from the response we return. This is important because the response we provide will be processed by the remaining functions, so we need to understand how they use it.

**Step 4 - Attach our implementation to `http.Client`**
After implementing the interface, we can assign our structure as the `Transport` of an `http.Client`.
`Transport` is the component that allows us to intercept outgoing HTTP requests by providing our own `RoundTripper` implementation.

**Step 5 - What happens next?**
When `*oauth2.Config.Exchange()` sends its HTTP request through the configured client, the request will reach our custom `RoundTrip()` method instead of the default transport.
This allows us to handle the request ourselves, without actually sending it to GitHub and failing because we haven't provided a real authorization code.

## What I Still Don't Understand

//
