---
description: Every key binding in the vlx explorer.
---

# Keys

The bottom line of the explorer always shows the keys that work right now;
++"?"++ (or ++f1++ anywhere) shows this list.

## Anywhere

| Key | Action |
| --- | --- |
| ++esc++ | go up a level (or stop a running stream) |
| ++f1++ | help |
| ++ctrl+k++ | go to the command line |
| ++ctrl+c++ | quit |

## Lists (services, a service)

| Key | Action |
| --- | --- |
| ++up++ / ++down++ | move |
| ++page-up++ / ++page-down++ | page up / down |
| ++home++ / ++end++ | top / bottom |
| ++enter++ / ++right++ | open |
| ++space++ | expand or collapse an interface |
| ++esc++ / ++left++ | up a level |
| *a letter* | start typing on the command line |
| ++"/"++ / ++":"++ | go to the command line |
| ++ctrl+d++ | documentation and IDL of the selection |
| ++ctrl+y++ | copy a `vlx call` command, a name or a socket path |
| ++ctrl+r++ | rediscover sockets and re-read the service |
| ++"?"++ | help |
| ++q++ | quit |

## Command line

| Key | Action |
| --- | --- |
| *type* | search, or write a call: `SERVICE [METHOD] [NAME=VALUE …]` |
| ++up++ ++down++ | pick a match · earlier calls |
| ++tab++ | complete the selected match |
| ++enter++ | go to the match, or run the call |
| ++esc++ | leave the command line |

## Method page

| Key | Action |
| --- | --- |
| ++ctrl+s++ | validate and send |
| ++tab++ / ++shift+tab++ | form → command line → result, and back |
| ++up++ ++down++ | (form) previous / next parameter |
| ++left++ ++right++ / ++space++ | (form) change a choice |
| ++backspace++ | (form) unset an optional choice |
| ++ctrl+j++ | switch between the form and JSON |
| ++ctrl+t++ | toggle `more` (streaming) |
| ++ctrl+o++ | toggle `oneway` |
| ++ctrl+r++ | reset the parameters |
| ++ctrl+y++ | copy the `vlx call` and `varlinkctl call` commands |
| ++ctrl+d++ | the method's full documentation |
| ++page-up++ ++page-down++ | scroll the result |
| ++y++ | (result) copy the replies as JSON |
| ++shift+r++ | (result) reveal redacted values |
| ++esc++ | stop a stream · back to the form · up a level |

Text fields and the JSON editor also support the usual editing keys:
++ctrl+a++ / ++ctrl+e++ for line start and end, ++ctrl+u++ to delete before
the cursor, ++alt+left++ / ++alt+right++ to move by word.

## Documentation and IDL

| Key | Action |
| --- | --- |
| ++up++ ++down++ ++page-up++ ++page-down++ | scroll |
| ++ctrl+d++ | switch between docs and IDL |
| ++esc++ | back |

## Confirmations

| Dialog | Keys |
| --- | --- |
| WRITE | ++y++ sends · ++n++ or ++esc++ cancels |
| DANGER | type the method name, then ++enter++ · ++esc++ cancels |
