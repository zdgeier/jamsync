import { basicSetup, EditorView } from "codemirror";
// import {StreamLanguage} from "@codemirror/language"
import { keymap } from "@codemirror/view";
import { indentWithTab } from "@codemirror/commands";
import { StreamLanguage } from "@codemirror/language";
import {go} from "@codemirror/legacy-modes/mode/go"
import {tags} from "@lezer/highlight"
import {HighlightStyle} from "@codemirror/language"
import {syntaxHighlighting} from "@codemirror/language"
import { vim } from "@replit/codemirror-vim";
// In your extensions...

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

const splitPath = window.location.pathname.split("/");
const projectName = splitPath[2];
const currentPath = splitPath.slice(4).join("/");

async function getDocument() {
  let projectMetadataResp = await fetch(`/api/committedchanges/${projectName}`);
  let projectMetadataJson = await projectMetadataResp.json();
  let maxCommitId = Math.max(...projectMetadataJson.change_ids);

  let fileResp = await fetch(
    `/api/projects/${projectName}/file/${currentPath}?commitId=${maxCommitId}`,
  );
  let fileText = await fileResp.text();

  return { maxCommitId, fileText };
}

async function populateEditor() {
  let { maxCommitId, fileText } = await getDocument();

  let extensions = [
    vim(),
    myTheme,
    basicSetup,
    keymap.of([indentWithTab]),
  ]
  if (currentPath.endsWith('.go')) {
    extensions.push(StreamLanguage.define(go))
    extensions.push(syntaxHighlighting(myHighlightStyle))
  }

  return new EditorView({
    doc: fileText,
    extensions: extensions,
    parent: document.getElementById("js-code-editor-location")!,
  });
}
populateEditor();


// import {Update, receiveUpdates, sendableUpdates, collab, getSyncedVersion} from "@codemirror/collab"
// import {basicSetup} from "codemirror"
// import {ChangeSet, EditorState, Text} from "@codemirror/state"
// import {EditorView, ViewPlugin, ViewUpdate} from "@codemirror/view"
// 
// ;(document.querySelector("#addpeer") as HTMLButtonElement).onclick = addPeer
// 
// function pause(time: number) {
//   return new Promise<void>(resolve => setTimeout(resolve, time))
// }
// 
// function currentLatency() {
//   let base = +(document.querySelector("#latency") as HTMLInputElement).value
//   return base * (1 + (Math.random() - 0.5))
// }
// 
// class Connection {
//   private disconnected: null | {wait: Promise<void>, resolve: () => void} = null
// 
//   constructor(private worker: Worker,
//               private getLatency: () => number = currentLatency) {}
// 
//   private _request(value: any): Promise<any> {
//     return new Promise(resolve => {
//       let channel = new MessageChannel
//       channel.port2.onmessage = event => resolve(JSON.parse(event.data))
//       this.worker.postMessage(JSON.stringify(value), [channel.port1])
//     })
//   }
// 
//   async request(value: any) {
//     let latency = this.getLatency()
//     await (this.disconnected ? this.disconnected.wait : pause(latency))
//     let result = await this._request(value)
//     await (this.disconnected ? this.disconnected.wait : pause(latency))
//     return result
//   }
// 
//   setConnected(value: boolean) {
//     if (value && this.disconnected) {
//       this.disconnected.resolve()
//       this.disconnected = null
//     } else if (!value && !this.disconnected) {
//       let resolve, wait = new Promise<void>(r => resolve = r)
//       this.disconnected = {wait, resolve}
//     }
//   }
// }
// 
// //!wrappers
// 
// function pushUpdates(
//   connection: Connection,
//   version: number,
//   fullUpdates: readonly Update[]
// ): Promise<boolean> {
//   // Strip off transaction data
//   let updates = fullUpdates.map(u => ({
//     clientID: u.clientID,
//     changes: u.changes.toJSON()
//   }))
//   return connection.request({type: "pushUpdates", version, updates, projectName, currentPath})
// }
// 
// function pullUpdates(
//   connection: Connection,
//   version: number
// ): Promise<readonly Update[]> {
//   return connection.request({type: "pullUpdates", version})
//     .then(updates => updates.map(u => ({
//       changes: ChangeSet.fromJSON(u.changes),
//       clientID: u.clientID
//     })))
// }
// let host = window.location.host;
// let protocol = window.location.protocol;
// let splitPath = self.location.pathname.split("/");
// let projectName = splitPath[2];
// let currentPath = splitPath.slice(4).join("/");
// 
// function getDocument(
//   connection: Connection
// ): Promise<{version: number, doc: Text}> {
//   return connection.request({type: "getDocument", protocol, host, projectName, currentPath}).then(data => ({
//     version: data.version,
//     doc: Text.of(data.doc.split("\n"))
//   }))
// }
// 
// //!peerExtension
// 
// let url = `ws:/\/${window.location.host}/api/ws/committedchanges/${projectName}`;
// if (window.location.protocol == "https:") {
//   url = `wss:/\/${window.location.host}/api/ws/committedchanges/${projectName}`;
// }
// let ws = new WebSocket(url);
// function peerExtension(startVersion: number, connection: Connection) {
//   let plugin = ViewPlugin.fromClass(class {
//     private pushing = false
//     private done = false
// 
//     constructor(private view: EditorView) {
//         //this.pull()
//         console.log(ws)
//         ws.onmessage = (e) => this.pull(e)
//     }
// 
//     update(update: ViewUpdate) {
//       if (update.docChanged) this.push()
//     }
// 
//     async push() {
//       let updates = sendableUpdates(this.view.state)
//       if (this.pushing || !updates.length) return
//       this.pushing = true
//       let version = getSyncedVersion(this.view.state)
//       await pushUpdates(connection, version, updates)
//       this.pushing = false
//       // Regardless of whether the push failed or new updates came in
//       // while it was running, try again if there's updates remaining
//       if (sendableUpdates(this.view.state).length)
//         setTimeout(() => this.push(), 100)
//     }
// 
//     async pull(updates) {
//         let version = getSyncedVersion(this.view.state)
//         console.log(updates)
//         this.view.dispatch(receiveUpdates(this.view.state, updates))
//     }
// 
//     //async pull() {
//     //  while (!this.done) {
//     //    let version = getSyncedVersion(this.view.state)
//     //    let updates = await pullUpdates(connection, version)
//     //    this.view.dispatch(receiveUpdates(this.view.state, updates))
//     //  }
//     //}
// 
//     destroy() { this.done = true }
//   })
//   return [collab({startVersion}), plugin]
// }
// 
// //!rest
// 
// const worker = new Worker("/public/worker.bundle.js");
// 
// async function addPeer() {
//   let {version, doc} = await getDocument(new Connection(worker, () => 0))
//   let connection = new Connection(worker)
//   let state = EditorState.create({
//     doc,
//     extensions: [basicSetup, peerExtension(version, connection)]
//   })
//   let editors = document.querySelector("#editors")
//   let wrap = editors.appendChild(document.createElement("div"))
//   wrap.className = "editor"
//   let cut = wrap.appendChild(document.createElement("div"))
//   cut.innerHTML = "<label><input type=checkbox aria-description='Cut'>✂️</label>"
//   cut.className = "cut-control"
//   cut.querySelector("input").addEventListener("change", e => {
//     let isCut = (e.target as HTMLInputElement).checked
//     wrap.classList.toggle("cut", isCut)
//     connection.setConnected(!isCut)
//   })
//   new EditorView({state, parent: wrap})
// }
// 
// addPeer()