# elector

a general purpose flexible leader election utility

## about

_elector_ is a general purpose leader election utility, providing a pluggable backend, as well as a flexible handlers for election events.

### cli

The cli for _elector_ accepts a `leader-begin-command` and `leader-end-command`; the former executed during transitions _to_ leader state, and the latter during transitions _from_ leader state. (`leader-begin-command` and `leader-end-command` are shell commands)

### library

The _elector_ library defines two basic interfaces; `ElectionBackend`, a backend that sources leader election events from a backend system, and `Handler`, a function executed after state transitions to perform arbitrary commands.

An `ElectorBackend` defines `ElectionLoop`, which is not expected to return, barring unrecoverable errors. It should continue to source election events and send `elector.State` messages to the `updates` channel (`StateLeader`, `StateNotLeader`, `StateError`). The reconciliation loop handles the transitions between states and executes the necessary handlers.

A `Handler` is executed by the reconciliation loop when state transitions occur _to_ and _from_ leader state. It is a function that accepts no arguments and returns and `error` in case it cannot complete it's task. In case of an error, the reconciliation loop will transition to `StateError`, ensuring that the _elector_ relinquishes master state and waits a given timeout before resuming candidacy.

#### extending / usage

To define an `ElectorBackend` take a look at the [console](backends/console.go) implementation - it is very straightforward.

To define custom handlers take a look at the pre-defined [handlers](handlers/handlers.go), also very straightforward.

For custom usage of an elector, take a look at it's usage in [main](main.go#L137). Create an `elector.Elector`, define it's backend and handlers, and call `Run()`.

## backends

### etcd-lock

The `etcd-lock` backend is supported by [etcd-lock](https://github.com/datawisesystems/etcd-lock). It acquires a lock on a node (`keyspace`) in etcd and continually updates the ttl on that node to maintain leader state. It supports full tls security.

### console

The `console` backend is used for testing proper execution of state transitions and handlers. It sources election events from the console, i.e. your terminal. It allows input of text events (`LEADER`, `NOTLEADER`, `ERROR`).

## reconciliation loop logic

Because _elector_ manages local state with the state of a remote leader election, it requires logic to reconcile local state machine with messages from the remote system. The reconciliation loop behaves as follows:

```
    message rcvd:
		case StateLeader:
			if state == StateNotLeader
				err = exec BeginLeaderHandler
				if not err
					state = StateLeader
				if err
					<- StateError

		case StateNotLeader:
			if state == StateLeader
				err = exec EndLeaderHandler
				if not err
					state = StateNotLeader
				if err
					<- StateError

		case Error:
			if state == StateLeader
				err = exec EndLeaderHandler
				if not err
					state = StateError
					exec ErrorHandler (timeout)
					state = StateNotLeader
				if err
					<- StateError
```

__WARNING__: after a `StateError` message, if the `EndLeaderHandler` fails repeatedly, there is a condition where the local system may continue to operate as leader (as is the case with any failure of the `EndLeaderHandler`). The loop will continue to retry every 5s for 12 attempts, after which it will exit the system and log failure messages.
