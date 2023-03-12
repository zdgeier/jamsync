import { keymap } from "@codemirror/view";
import { indentWithTab } from "@codemirror/commands";
import { StreamLanguage } from "@codemirror/language";
import {go} from "@codemirror/legacy-modes/mode/go"
import {tags} from "@lezer/highlight"
import {HighlightStyle} from "@codemirror/language"
import {syntaxHighlighting} from "@codemirror/language"
import { vim } from "@replit/codemirror-vim";
import {Update, receiveUpdates, sendableUpdates, collab, getSyncedVersion} from "@codemirror/collab"
import {basicSetup} from "codemirror"
import {ChangeSet, EditorState, Text} from "@codemirror/state"
import {EditorView, ViewPlugin, ViewUpdate} from "@codemirror/view"

const splitPath = window.location.pathname.split("/");
const projectName = splitPath[2];
const currentPath = splitPath.slice(4).join("/");

const myHighlightStyle = HighlightStyle.define([
  {tag: tags.keyword, color: "var(--tan)"},
  {tag: tags.comment, color: "#00ff80"},
  {tag: tags.string, color: "var(--bright-blue)"}
])

// .cm-editor.cm-focused { outline: 2px solid cyan }
// .cm-editor .cm-content { font-family: "Consolas" }

let myTheme = EditorView.theme({
  "&": {
    color: "white",
    backgroundColor: "var(--dark-blue)",
    font: "'Fira Code', Monaco, Consolas, Ubuntu Mono, monospace",
  },
  ".cm-editor.cm-focused": {
    outline: "1px solid var(--bright-pink)",
  },
  ".cm-content": {
    caretColor: "white",
  },
  ".cm-activeLineGutter": {
    backgroundColor: "rgba(255, 0, 127, 0.25)",
  },
  ".cm-activeLine": {
    backgroundColor: "rgba(255, 0, 127, 0.25)",
  },
  "&.cm-focused .cm-cursor": {
    borderLeftColor: "var(--bright-pink)",
  },
  "&.cm-focused .cm-selectionBackground, ::selection": {
    backgroundColor: "#ff007f",
  },
  ".cm-selectionMatch": {
    backgroundColor: "#ff007f",
  },
  ".cm-gutters": {
    backgroundColor: "var(--dark-blue)",
    color: "#ddd",
    border: "none",
  },
}, { dark: true });

async function getDocument() {
  let projectMetadataResp = await fetch(`/api/committedchanges/${projectName}`);
  let projectMetadataJson = await projectMetadataResp.json();
  let maxCommitId = Math.max(...projectMetadataJson.change_ids);

  let fileResp = await fetch(
    `/api/projects/${projectName}/file/${currentPath}?commitId=${maxCommitId}`,
  );
  let fileText = await fileResp.text();

  return { version: maxCommitId, doc: fileText };
}

function peerExtension(startVersion: number) {
  let plugin = ViewPlugin.fromClass(class {
    private pushing = false
    private done = false
    private ws: WebSocket = null;

    constructor(private view: EditorView) {
      let url = `ws:/\/${window.location.host}/api/ws/editorchanges/${projectName}`;
      if (window.location.protocol == "https:") {
        url = `wss:/\/${window.location.host}/api/ws/editorchanges/${projectName}`;
      }
      this.ws = new WebSocket(url);
      this.ws.onmessage = (event) => {
        console.log("RECEIVED", event.data)
        event.data
        let strippedUpdates = JSON.parse(event.data).map(u => ({
          changes: ChangeSet.fromJSON(u.changes),
          clientID: u.clientID
        }));
        this.view.dispatch(receiveUpdates(this.view.state, strippedUpdates))
      }
    }

    update(update: ViewUpdate) {
      if (update.docChanged) this.push()
    }

    async push() {
      let updates = sendableUpdates(this.view.state)
      if (this.pushing || !updates.length) return
      this.pushing = true
      let version = getSyncedVersion(this.view.state)
      let update = {version, updates}
      console.log("SENDING", update)
      let strippedUpdates = updates.map(u => ({
        clientID: u.clientID,
        changes: u.changes.toJSON()
      }))
      this.ws.send(JSON.stringify(strippedUpdates))
      this.pushing = false
      if (sendableUpdates(this.view.state).length)
        setTimeout(() => this.push(), 100)
      fetch(`/api/projects/${projectName}/file/${currentPath}`, {method: 'PUT', body: this.view.state.doc.toString()});
    }

    destroy() { this.done = true }
  })
  return [collab({startVersion}), plugin]
}

async function addPeer() {
  let {version, doc} = await getDocument()
  console.log("Version:", version)
  let extensions = [
    vim(),
    myTheme,
    basicSetup,
    keymap.of([indentWithTab]),
    peerExtension(version)
  ]
  if (currentPath.endsWith('.go')) {
    extensions.push(StreamLanguage.define(go))
    extensions.push(syntaxHighlighting(myHighlightStyle))
  }
  let state = EditorState.create({
    doc,
    extensions: extensions,
  })
  let editors = document.querySelector("#editors")
  let wrap = editors.appendChild(document.createElement("div"))
  wrap.className = "editor"
  new EditorView({state, parent: wrap})
}

addPeer()