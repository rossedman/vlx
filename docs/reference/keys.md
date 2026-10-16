---
description: Every key binding in the vlx explorer.
---

# Keys

The bottom line of the explorer always shows the keys that work right now;
++"?"++ shows this list.

## Browse

| Key | Action |
| --- | --- |
| ++up++ / ++k++ | up |
| ++down++ / ++j++ | down |
| ++page-up++ / ++page-down++ | page up / down |
| ++g++ / ++shift+g++ | top / bottom |
| ++tab++ / ++shift+tab++ | next / previous pane |
| ++left++ ++right++ / ++h++ ++l++ | previous / next pane |
| ++enter++ | open service · expand interface · call method |
| ++space++ | expand or collapse an interface |
| ++c++ | call the selected method |
| ++i++ | toggle the raw IDL view |
| ++"/"++ | filter the focused list |
| ++y++ | copy the `vlx call` command (or the name) |
| ++r++ | refresh sockets and the open service |
| ++esc++ | back: close IDL, leave filter, previous pane |
| ++"?"++ | help |
| ++q++ / ++ctrl+c++ | quit |

## Filter

| Key | Action |
| --- | --- |
| *type* | narrow the list as you type |
| ++enter++ | keep the filter and return to the list |
| ++esc++ | clear the filter |

## Call

| Key | Action |
| --- | --- |
| ++ctrl+s++ | validate and send |
| ++tab++ | switch between editor and response |
| ++ctrl+t++ | toggle `more` (streaming) |
| ++ctrl+o++ | toggle `oneway` |
| ++ctrl+r++ | reset parameters to the skeleton |
| ++ctrl+y++ | copy the `vlx call` and `varlinkctl call` commands |
| ++y++ | copy the replies as JSON (response focused) |
| ++shift+r++ | reveal redacted values (response focused) |
| ++up++ ++down++ ++g++ ++shift+g++ | scroll the response (response focused) |
| ++esc++ | stop a stream · back to the tree |
| ++f1++ | help |
| ++ctrl+c++ | quit |

The editor also supports the usual editing keys: ++ctrl+a++ / ++ctrl+e++
for line start and end, ++ctrl+k++ / ++ctrl+u++ to delete after or before
the cursor, ++alt+left++ / ++alt+right++ to move by word.

## Confirmations

| Dialog | Keys |
| --- | --- |
| WRITE | ++y++ sends · ++n++ or ++esc++ cancels |
| DANGER | type the method name, then ++enter++ · ++esc++ cancels |
