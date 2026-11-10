# D25. zuzu reads data only through a loader that the caller gives

Status: Decided.

The maintainers decided on 2026-11-10 that zuzu reads no file and makes no
request of its own. A spec that names its data by `url` needs a loader, which
the caller gives as an option. Without one, such a spec fails in either
feature mode.

So the caller decides what a spec can reach: a folder, a set of files in
memory, or the network. A spec from an untrusted source cannot make zuzu
read a file or call a server. `docs/API.md` sketches the `Loader` interface
and a loader that reads from an `fs.FS` that the caller chooses.
