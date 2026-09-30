# Why the schedule has the minimum number of turns

## The model

One tunnel crossing takes one turn. Intermediate rooms have capacity one. Start and end have unlimited occupancy, but each tunnel, including a direct start–end tunnel, has capacity one per turn. An ant may enter a room vacated in that same turn. The output orders each pipeline's older ants first, so a left-to-right execution of the tokens also respects vacancy.

Split every intermediate room into `room_in → room_out`, with capacity one and cost zero. A tunnel becomes a directed edge of capacity one and cost one in each useful direction. Edges into start and out of end are omitted: such detours cannot help a quickest single-source/single-sink schedule. The source is `start_out`; the sink is `end_in`.

A unit of static flow is a route. Room splitting makes integral routes internally vertex-disjoint. Positive tunnel costs mean a minimum-cost flow contains no positive-cost cycles. In particular, it never needs both directions of an undirected tunnel: those would form a removable positive-cost cycle. A direct start–end edge also has capacity one.

## Successive shortest augmentations

Starting from zero flow, add one unit at a time using a cheapest augmenting path. A reverse edge cancels earlier routing choices; this is what a greedy BFS followed by deleting used rooms cannot do. After augmentation `k`, the result is a minimum-cost integral flow of value `k`.

Reverse edges have negative original costs. Dijkstra is run on **reduced costs**:

```text
reduced_cost(u, v) = original_cost(u, v) + potential[u] - potential[v]
```

All initially usable edges have nonnegative costs. After a shortest-path search, add the computed distance to each reachable vertex's potential. This keeps every reachable residual edge's reduced cost nonnegative. Newly opened reverse edges on the chosen path have reduced cost zero. The queue breaks equal distances by vertex ID, and paths are sorted by length and room name. No route selection depends on Go map iteration.

The marginal cost of another unit of flow is `potential[sink] - potential[source]`; these costs do not decrease.

## From routes to a deadline

For a route with `d` tunnels, launch one ant per turn. By the end of turn `T`, it can deliver:

```text
max(0, T - d + 1)
```

The first arrival is on turn `d`; subsequent arrivals are one turn apart. Add the capacities of the independent routes. Binary search finds the smallest `T` for which they can deliver all `N` ants. Allocate no more than each route's capacity at that deadline. Unused routes are omitted.

Example: routes of lengths 2 and 4, with 8 ants. At turn 5 they can deliver `4 + 2 = 6`; at turn 6 they can deliver `5 + 3 = 8`. Six turns suffice. The longest route is not filled just because it exists.

## Why considering these flows is sufficient

The underlying result is the single-source/single-sink **maximum flow over time** theorem: an optimum can be built by repeating suitable static path flows, without waiting at intermediate vertices. See Ford and Fulkerson, [Constructing Maximal Dynamic Flows from Static Flows (1958)](https://pubsonline.informs.org/doi/10.1287/opre.6.3.419).

Here is how that result applies to this program, rather than assuming that arbitrary disjoint paths are enough:

1. At a fixed deadline `T`, augment the static split-room network with an end-to-start return edge of cost `-(T+1)`. Minimizing circulation cost maximizes the number deliverable by `T`.
2. Sending `k` units around this circulation has value `k*(T+1) - C(k)`, where `C(k)` is the minimum total route length for flow value `k`. Successive shortest augmentations compute these `C(k)` values.
3. Only profitable routes, with length at most `T`, are repeated. A negative contribution from a too-long route can simply be omitted. This is why the implementation evaluates actual path capacities using `max(0, ...)`, rather than assuming every chosen path contributes.
4. Any legal original schedule is feasible in the directed flow-over-time relaxation with unlimited waiting at split vertices. The theorem therefore provides an upper bound on original arrivals. The repeated integral paths produced here share neither intermediate rooms nor tunnels, require no intermediate waiting, and attain that bound within the original constraints.
5. Integral unit room capacities make useful repeated routes internally disjoint. The directed relaxation may permit opposing tunnel directions generally; its optimum has no positive-cost cycles, so the repeated solution never violates the original undirected tunnel capacity.

Thus some flow value examined by the solver achieves an optimal deadline. At most `N` useful unit paths are needed for `N` ants.

The solver can stop early once the next marginal path cost `d` is at least the best known deadline `B`. At the only relevant shorter horizon `B-1`, that flow increment contributes `B-d <= 0`. Later marginal costs cannot be smaller, so no later flow improves the deadline. If there is no augmenting path at all, start and end are disconnected and the program returns an error.

## A trap for greedy path deletion

In `examples/reroute.txt`, the first shortest path is `s-a-b-t` (3 tunnels). Keeping it forever leaves no second independent route. Reverse residual edges allow replacing it with:

```text
s-a-c-d-t     (4 tunnels)
s-e-f-b-t     (4 tunnels)
```

For 12 ants, each route carries 6 ants in `4 + 6 - 1 = 9` turns. The isolated shortest path would need `3 + 12 - 1 = 14` turns.

## Memory, work, and cancellation

Let `V` be rooms, `E` tunnels, `F` examined unit flows, `M` printed movements, and `K` selected routes.

- Network storage is `O(V+E)`; queue entries can be `O(E)`.
- Each augmentation takes `O((V+E) log(V+E))` with the lazy binary heap; decomposition is linear in the graph size in the worst case.
- Evaluating a deadline takes `O(K log(N+V))`.
- The streaming plan takes `O(V+K)` memory, independently of the number of output movements. No array of all ant positions or all turns is needed by the solver.
- Emission takes `O(M + K*T)` plus the bytes of room names written. An unavoidable large transcript takes correspondingly long to print.
- Input, names and graph size are explicitly bounded; integer counts are validated before graph construction.
- Ctrl+C/SIGTERM cancels graph searches and output loops. Ordinary parser, solver and writer failures become error messages, not panics.

The visualizer has separate transcript/movement limits because it must hold a replay in memory. Its constraints do not reduce the solver's accepted ant count.

## Independent evidence

`internal/routing/solver_test.go` includes a breadth-first oracle that enumerates all legal joint moves, including waiting and returning to start. It does not use residual networks, route lengths, potentials or the production allocator. States differing only by ant identity are equivalent for this purpose.

Every simple graph with 2–5 rooms is checked with 1–3 ants: 3294 cases, including disconnected graphs. Another 250 fixed-seed graphs have 6–7 rooms and up to four ants. Every generated output is checked by `internal/replay`, and an additional left-to-right validator ensures receiving rooms have already been vacated. Exhaustive small-graph testing is evidence alongside the flow argument, not a claim that finite tests prove all possible inputs.
