## `zerolog` Instead of `slog`

### Why This Choice?
* At first, I didn't really care which logging library I was going to use because I thought it wasn't a big deal. And honestly, it wasn't. However, I was looking for a fast logging library because I wanted to make everything as efficient as possible. This is also one of the reasons I chose `sonic` over Go's standard `encoding/json` package. After learning more about `zerolog`, I found it simpler and less verbose than `slog`, at least in my experience. I had also seen recommendations for it on Reddit and other platforms, so I decided to give it a try. I ended up genuinely enjoying it! However, there are a few significant drawbacks that became more noticeable as the project grew.

* **Pros:**
* * **Simplicity** -> Even though I had only worked with `slog` before, I quickly understood how `zerolog` was structured. Its logging levels are easy to identify through `zerolog.Level`, and setting it up requires very little configuration. Overall, I found its API comfortable to work with. It also comes with a built-in context process (pretty handy)
* * **Performance** -> I cannot honestly claim that `zerolog` is thousands of times faster than `slog`, because I have not conducted any benchmarks to support that statement. Although performance was one of the reasons I considered it, my decision was influenced more by community recommendations and the simplicity of its API than by measured performance differences.

* **Cons:**
* * **Redundancy** -> Logging with `zerolog` typically requires calling `.Msg()` at the end of a logging chain. It is easy to forget this final step, which can lead to incomplete logging statements or compilation errors. Although this is a small inconvenience, it becomes noticeable when writing logging code frequently.
* * **Ecosystem** -> This was the most significant drawback for me. Since OpenTelemetry is widely used for observability, I wanted to experiment with integrating it into my project. However, getting everything working with `zerolog` proved more difficult than I expected. Compared with the more straightforward integration options available around `slog`, the process felt unnecessarily complicated.

Ultimately, I chose `zerolog` because I enjoyed working with it and appreciated its simplicity. However, choosing a library is not just about its API or performance; its ecosystem and integration with other tools matter just as much.

## `sonic` Instead of `encoding/json`
### Why This Choice?
This was a much easier choice once I understood why `sonic` was so fast. Lemme explain the difference:
* **`encoding/json`** -> Go's standard JSON library uses reflection to figure out how to handle your structs and their fields at runtime. It works perfectly fine, but this process can add some overhead.
* **`sonic`** -> `sonic` uses some optimizations, including JIT compilation, to make JSON encoding and decoding faster. Basically, it can prepare optimized code for handling your types instead of doing all the work the same way every time.

* **Pros:**
* * **Speed** -> This was the main reason I chose it. I wanted something fast, and `sonic` seemed like a good choice for that. I haven't benchmarked it against `encoding/json` in my own project, though, so I can't say how much of a difference it actually makes.
* * **Familiarity** -> Even though `sonic` is a different library, it feels pretty similar to `encoding/json`. This made it really easy to start using without having to learn an entirely different way of handling JSON.
* * **Simplicity** -> I didn't have to change much of my code to start using it. I could keep working with JSON almost the same way as before, just with a different library.

* **Cons:**
* * **Extra Dependency** -> Unlike `encoding/json`, `sonic` is an external dependency. This means I have another library to maintain and consider when updating my project.
* * **Compatibility** -> Even though `sonic` feels similar to `encoding/json`, there can still be differences in how they behave in certain situations. I need to keep that in mind instead of assuming they work exactly the same way.
* * **Unproven Performance** -> I chose `sonic` because of its speed, but I haven't tested it against `encoding/json` in my own project. So, while I like the idea of having a faster JSON library, I still need to check whether it actually makes a noticeable difference for my use case.

Honestly, this is probably one of my favorite choices so far! I really enjoy working with `sonic`, and `encoding/json` was pretty comfy, too, since the APIs are so similar. I simply wanted to try something that focuses more on performance, and `sonic` seemed like a good fit.