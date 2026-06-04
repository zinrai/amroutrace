# amroutrace

An end-to-end trace for Alertmanager routing.

## What it does

amroutrace checks which receivers an alert reaches when routed by a real
Alertmanager, and compares that against the receivers declared in a test
specification. The specification is the same `routing_tests.yml` consumed by
[amroutify](https://github.com/zinrai/amroutify), so the static check and the
end-to-end check assert against one source of truth.

The check is framed as: for a given input (a set of labels), which receivers
does the alert pass through. Firing versus resolved is not distinguished; a
receiver is either passed through or not. This matches how amroutify evaluates
routing statically.

The tool has three operations:

- `rewrite` rewrites a rendered production config so every receiver delivers to
  a local capture endpoint, and writes it to `amroutrace-<name>`. It is a dry
  run of inject's front half: useful for inspecting exactly what inject will
  feed to Alertmanager.
- `inject` does the same rewrite, then starts its own Alertmanager with the
  rewritten config and posts each test case's alert. Because it runs its own
  Alertmanager, it never touches a production instance.
- `verify` reads an arrivals file describing which receiver each alert reached,
  and compares the set of receivers reached against `expected_receivers`.

amroutrace does not read any capture tool's log directly. It reads a neutral
arrivals format, so the tool that receives the webhooks can be replaced without
changing amroutrace.

## The rewritten config

Both `rewrite` and `inject` turn the input config into a webhook-only config
written to `amroutrace-<base name>` in the current directory. For a given input
the output name is fixed, so repeated runs overwrite a single file. Every
receiver is rewritten to deliver to `<capture-url>/<receiver-name>`; the route
tree and receiver names are unchanged, so routing behavior is preserved.
Notification parameters such as `send_resolved` are dropped: routing is
determined by labels and the route tree, not by how a receiver delivers.

`inhibit_rules` are removed. Inhibition suppresses one alert's notification
when another is firing, which is separate from routing; left in place it could
stop an alert from reaching a receiver it routes to and cause a false "not
delivered". amroutrace verifies which receivers an input passes through, not
inhibition.

inject leaves this file in place after the run, as a record of what was fed to
Alertmanager.

## Running an end-to-end test

1. Render the production `alertmanager.yml` to test (for example with Ansible),
   without any test-specific changes.
2. Start the capture endpoint that records the received webhooks, listening
   where `-capture-url` points (default `http://127.0.0.1:8000`).
3. Inject. amroutrace writes the rewritten config, starts its own Alertmanager,
   posts the alerts, and keeps Alertmanager running so that deliveries reach the
   capture endpoint:

   ```
   $ amroutrace inject \
       -tests routing_tests.yml \
       -config alertmanager.yml \
       -alertmanager-bin /usr/local/bin/alertmanager
   wrote amroutrace-alertmanager.yml
   injected: single receiver
   injected: fan out via continue
   ...

   injected 4 case(s); Alertmanager running at http://127.0.0.1:9093
   running until interrupted (Ctrl-C)
   ```

   Alertmanager runs until you press Ctrl-C, or for a fixed duration if you
   pass `-run-for`. Wait long enough for the webhooks to be delivered
   (governed by the config's `group_wait`) before stopping.
4. Convert the capture endpoint's log to the arrivals format (see below) and
   verify:

   ```
   $ amroutrace verify -tests routing_tests.yml -arrivals arrivals.jsonl | jq .
   {
     "passed": false,
     "total": 4,
     "results": [
       {
         "name": "single receiver",
         "passed": false,
         "expected": ["receiver-a"],
         "actual": ["receiver-b"],
         "missing": ["receiver-a"],
         "unexpected": ["receiver-b"]
       }
     ]
   }
   ```

`verify` exits non-zero when any case fails, so it can gate a CI pipeline on the
exit code without parsing the JSON.

## Arrivals format

`verify` reads a JSON-lines file: one delivery per line, each an object with a
`receiver` field (the receiver the alert reached) and a `case` field (the test
case name, matching `name` in `routing_tests.yml`).

```
{"receiver":"receiver-a","case":"single receiver"}
{"receiver":"receiver-a","case":"fan out via continue"}
{"receiver":"receiver-b","case":"fan out via continue"}
```

A case that reaches several receivers appears on several lines. A case expected
to reach no receiver simply does not appear. The receiver name is taken from
the Alertmanager webhook payload's `receiver` field, so receivers sharing a URL
are still distinguished.

Producing this file is the caller's responsibility, which keeps amroutrace
independent of any particular capture tool.

## Options

### rewrite

```
-config       Path to the rendered production alertmanager.yml to rewrite (required)
-capture-url  Base URL of the capture endpoint; receivers are rewritten to
              <capture-url>/<receiver-name> (default http://127.0.0.1:8000)
```

### inject

```
-tests            Path to routing_tests.yml (required)
-config           Path to the rendered production alertmanager.yml to test (required)
-alertmanager-bin Path to the alertmanager binary (default "alertmanager")
-listen           Address Alertmanager listens on (default 127.0.0.1:9093)
-capture-url      Base URL of the capture endpoint; receivers are rewritten to
                  <capture-url>/<receiver-name> (default http://127.0.0.1:8000)
-ready-timeout    How long to wait for Alertmanager to become ready (default 10s)
-timeout          Per-request HTTP timeout (default 1s)
-run-for          Stop Alertmanager after this duration; 0 means run until
                  interrupted (default 0)
```

### verify

```
-tests     Path to routing_tests.yml (required)
-arrivals  Path to the arrivals file: JSON lines, one delivery per line, each
           mapping a receiver to a test case (required)
```

## Test specification

The test specification (`routing_tests.yml`) uses the same format as
[amroutify](https://github.com/zinrai/amroutify); see that repository for the
format. An empty `expected_receivers` asserts the alert reaches no receiver,
and receiver order does not matter.

## License

This project is licensed under the [MIT License](LICENSE).
