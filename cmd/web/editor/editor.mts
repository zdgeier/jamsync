import {
  collab,
  getSyncedVersion,
  receiveUpdates,
  sendableUpdates,
  Update,
} from "@codemirror/collab";
import { basicSetup } from "codemirror";
import { ChangeSet, EditorState, Text } from "@codemirror/state";
import { EditorView, keymap, ViewPlugin, ViewUpdate } from "@codemirror/view";
import { indentWithTab } from "@codemirror/commands";
import { vim } from "@replit/codemirror-vim";

// private _request(value: any): Promise<any> {
//   return new Promise((resolve) => {
//     let channel = new MessageChannel();
//     channel.port2.onmessage = (event) => resolve(JSON.parse(event.data));
//     this.worker.postMessage(JSON.stringify(value), [channel.port1]);
//   });
// }

// async request(value: any) {
//   return await this._request(value);
// }

// setConnected(value: boolean) {
//   if (value && this.disconnected) {
//     this.disconnected.resolve();
//     this.disconnected = null;
//   } else if (!value && !this.disconnected) {
//     let resolve, wait = new Promise<void>((r) => resolve = r);
//     this.disconnected = { wait, resolve };
//   }
// }

//!wrappers

// function pushUpdates(
//   connection: Connection,
//   version: number,
//   fullUpdates: readonly Update[],
// ): Promise<boolean> {
//   // Strip off transaction data
//   let updates = fullUpdates.map((u) => ({
//     clientID: u.clientID,
//     changes: u.changes.toJSON(),
//   }));
//   return connection.pushUpdates(version, updates)
// }

// function pullUpdates(
//   connection: Connection,
//   version: number,
// ): Promise<readonly Update[]> {
//   return connection.pullUpdates(version)
//   //return connection.request({ type: "pullUpdates", version })
//   //  .then((updates) =>
//   //    updates.map((u) => ({
//   //      changes: ChangeSet.fromJSON(u.changes),
//   //      clientID: u.clientID,
//   //    }))
//   //  );
// }

//!peerExtension

function peerExtension(startVersion: number, projectName: String) {
  let url =
    `ws:/\/${window.location.host}/api/ws/committedchanges/${projectName}`;
  let ws = new WebSocket(url);
  let plugin = ViewPlugin.fromClass(
    class {
      private pushing = false;

      constructor(private view: EditorView) {
        ws.onmessage = (event) => {
          console.log("message", event);
          let version = getSyncedVersion(this.view.state);
          //let updates = await connection.pullUpdates(version);
          this.view.dispatch(
            receiveUpdates(this.view.state, JSON.parse(event.data)),
          );
        };
      }

      update(update: ViewUpdate) {
        if (update.docChanged) this.push();
      }

      async push() {
        let updates = sendableUpdates(this.view.state).map((u) => ({
          clientID: u.clientID,
          changes: u.changes.toJSON(),
        }));
        if (this.pushing || !updates.length) return;
        this.pushing = true;
        let version = getSyncedVersion(this.view.state);
        console.log("sending")
        ws.send(JSON.stringify({ version, updates }));

        this.pushing = false;
        // Regardless of whether the push failed or new updates came in
        // while it was running, try again if there's updates remaining
        if (sendableUpdates(this.view.state).length) {
          setTimeout(() => this.push(), 100);
        }
      }
    },
  );
  return [collab({ startVersion }), plugin];
}

//!rest

//const worker = new Worker("/public/worker.bundle.js");

let updates: Update[] = [];
async function addPeer() {
  let splitPath = self.location.pathname.split("/");
  let projectName = splitPath[2];
  let currentPath = splitPath.slice(4).join("/");
  let projectMetadataResp = await fetch(
    `/api/committedchanges/${projectName}`,
  );
  let projectMetadataJson = await projectMetadataResp.json();
  let maxCommitId = Math.max(...projectMetadataJson.change_ids);

  let fileResp = await fetch(
    `/api/projects/${projectName}/file/${currentPath}?commitId=${maxCommitId}`,
  );
  let doc = Text.of((await fileResp.text()).split("\n"));

  let state = EditorState.create({
    doc,
    extensions: [basicSetup, peerExtension(updates.length, projectName)],
  });
  let editors = document.querySelector("#editors");
  let wrap = editors.appendChild(document.createElement("div"));
  wrap.className = "editor";
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
      backgroundColor: "var(--bright-pink)",
    },
    ".cm-selectionMatch": {
      backgroundColor: "var(--bright-pink)",
    },
    ".cm-gutters": {
      backgroundColor: "var(--dark-blue)",
      color: "#ddd",
      border: "none",
    },
  }, { dark: true });
  new EditorView({
    state,
    parent: wrap,
    extensions: [
      myTheme,
      basicSetup,
      keymap.of([indentWithTab]),
      vim(),
    ],
  });
}

(document.querySelector("#addpeer") as HTMLButtonElement).onclick = addPeer;

addPeer();

// import { basicSetup, EditorView } from "codemirror";
// // import {StreamLanguage} from "@codemirror/language"
// import { keymap, ViewPlugin, ViewUpdate } from "@codemirror/view";
// import { indentWithTab } from "@codemirror/commands";
// import { vim } from "@replit/codemirror-vim";
// import {
//   collab,
//   getSyncedVersion,
//   receiveUpdates,
//   sendableUpdates,
//   Update,
// } from "@codemirror/collab";
// import { ChangeSet } from "@codemirror/state";
// // import {go} from "@codemirror/legacy-modes/mode/go"
//
// // .cm-editor.cm-focused { outline: 2px solid cyan }
// // .cm-editor .cm-content { font-family: "Consolas" }
//
// let myTheme = EditorView.theme({
//   "&": {
//     color: "white",
//     backgroundColor: "var(--dark-blue)",
//     font: "'Fira Code', Monaco, Consolas, Ubuntu Mono, monospace",
//   },
//   ".cm-editor.cm-focused": {
//     outline: "1px solid var(--bright-pink)",
//   },
//   ".cm-content": {
//     caretColor: "white",
//   },
//   ".cm-activeLineGutter": {
//     backgroundColor: "rgba(255, 0, 127, 0.25)",
//   },
//   ".cm-activeLine": {
//     backgroundColor: "rgba(255, 0, 127, 0.25)",
//   },
//   "&.cm-focused .cm-cursor": {
//     borderLeftColor: "var(--bright-pink)",
//   },
//   "&.cm-focused .cm-selectionBackground, ::selection": {
//     backgroundColor: "var(--bright-pink)",
//   },
//   ".cm-selectionMatch": {
//     backgroundColor: "var(--bright-pink)",
//   },
//   ".cm-gutters": {
//     backgroundColor: "var(--dark-blue)",
//     color: "#ddd",
//     border: "none",
//   },
// }, { dark: true });
//
// function pushUpdates(
//   version: number,
//   fullUpdates: readonly Update[],
// ): Promise<boolean> {
//   // Strip off transaction data
//   let updates = fullUpdates.map((u) => ({
//     clientID: u.clientID,
//     changes: u.changes.toJSON(),
//   }));
//
//   console.log("push updates", fullUpdates);
//   //return connection.request({ type: "pushUpdates", version, updates });
//   return new Promise<boolean>(resolve => resolve(true));
// }
//
// let splitPath = window.location.pathname.split("/");
// let projectName = splitPath[2];
// let url =
//   `wss:/\/${window.location.host}/api/ws/committedchanges/${projectName}`;
// let c = new WebSocket(url);
//
// function pullUpdates(
//   version: number,
// ): Promise<readonly Update[]> {
//   console.log("pull updates");
//   return new Promise<readonly Update[]>(resolve => {
//     c.addEventListener("message", (event) => resolve(JSON.parse(event.data)));
//   });
//   //return connection.request({ type: "pullUpdates", version })
//   //  .then((updates) =>
//   //    updates.map((u) => ({
//   //      changes: ChangeSet.fromJSON(u.changes),
//   //      clientID: u.clientID,
//   //    }))
//   //  );
// }
//
// async function getDocument() {
//   let splitPath = window.location.pathname.split("/");
//   let projectName = splitPath[2];
//   let currentPath = splitPath.slice(4).join("/");
//
//   let projectMetadataResp = await fetch(`/api/committedchanges/${projectName}`);
//   let projectMetadataJson = await projectMetadataResp.json();
//   let maxCommitId = Math.max(...projectMetadataJson.change_ids);
//
//   let fileResp = await fetch(
//     `/api/projects/${projectName}/file/${currentPath}?commitId=${maxCommitId}`,
//   );
//   let fileText = await fileResp.text();
//
//   return { maxCommitId, fileText };
// }
//
// function peerExtension(startVersion: number) {
//   let plugin = ViewPlugin.fromClass(
//     class {
//       private pushing = false;
//       private done = false;
//
//       constructor(private view: EditorView) {
//         this.pull();
//       }
//
//       update(update: ViewUpdate) {
//         if (update.docChanged) this.push();
//       }
//
//       async push() {
//         console.log("push");
//         let updates = sendableUpdates(this.view.state);
//         if (this.pushing || !updates.length) return;
//         this.pushing = true;
//         let version = getSyncedVersion(this.view.state);
//         await pushUpdates(version, updates);
//         this.pushing = false;
//         // Regardless of whether the push failed or new updates came in
//         // while it was running, try again if there's updates remaining
//         if (sendableUpdates(this.view.state).length) {
//           setTimeout(() => this.push(), 100);
//         }
//       }
//
//       async pull() {
//         while (!this.done) {
//           let version = getSyncedVersion(this.view.state);
//           let updates = await pullUpdates(version);
//           this.view.dispatch(receiveUpdates(this.view.state, updates));
//         }
//       }
//
//       destroy() {
//         this.done = true;
//       }
//     },
//   );
//   return [collab({ startVersion }), plugin];
// }
//
// async function populateEditor() {
//   let { maxCommitId, fileText } = await getDocument();
//
//   return new EditorView({
//     doc: fileText,
//     //extensions: [basicSetup, keymap.of([indentWithTab]), StreamLanguage.define(go)],
//     extensions: [
//       myTheme,
//       basicSetup,
//       keymap.of([indentWithTab]),
//       peerExtension(maxCommitId),
//       vim(),
//     ],
//     parent: document.getElementById("js-code-editor-location")!,
//   });
// }
// populateEditor();

// let splitPath = window.location.pathname.split("/");
// let projectUrl =  splitPath.slice(0, 3).join('/');
// let projectName = splitPath[2];
// let currentPath = splitPath.slice(4).join('/');
// let projectNameEl = document.getElementById("projectname");
// projectNameEl.innerHTML = "$ " + projectName + "/" + currentPath;
//
// function debounce(func, timeout = 1000){
//     let timer;
//     return (...args) => {
//         let wrapper = document.getElementById("js-file-location-wrapper");
//         wrapper.classList.add('Files-wrapper--editing');
//         clearTimeout(timer);
//         timer = setTimeout(() => { func.apply(this, args); }, timeout);
//     };
// }
//
// let put = async () => {
//     let outerDiv = document.getElementById("js-file-location");
//     let fileResp = await fetch(`/api/projects/${projectName}/file/${currentPath}`, {method: 'PUT', body: outerDiv.innerText});
//     let fileRespJson = await fileResp.json();
//     const queryParams = new URLSearchParams(window.location.search);
//     const commitIdInput = document.getElementById("commitIdInput");
//     commitIdInput.min = 1;
//     commitIdInput.max = fileRespJson.commitId;
//     commitIdInput.value = fileRespJson.commitId;
//     queryParams.set("commitId", fileRespJson.commitId);
//     history.replaceState(null, null, "?"+queryParams.toString());
//     let wrapper = document.getElementById("js-file-location-wrapper");
//     wrapper.classList.remove('Files-wrapper--editing');
// }
// let putNewFile = debounce(put)
//
// window.addEventListener('keydown', async (e) => {
//     if ((navigator.platform.match("Mac") ? e.metaKey : e.ctrlKey) && e.key == 's') {
//         e.preventDefault()
//         if (document.getElementById("js-file-location-wrapper").classList.contains('Files-wrapper--editing')) {
//             await put()
//             window.onbeforeunload = null;
//         }
//     }
// }, false)
//
// let startChange = () => {
//     let wrapper = document.getElementById("js-file-location-wrapper");
//     wrapper.classList.add('Files-wrapper--editing');
//     window.onbeforeunload = function() {
//         return true;
//     };
// }
//
// async function updateFile(commitId) {
//     let fileResp = await fetch(`/api/projects/${projectName}/file/${currentPath}?commitId=${commitId}`);
//     let fileText = await fileResp.text();
//
//
//     let outerDiv = document.getElementById("js-file-location");
//     outerDiv.removeEventListener('input', startChange);
//     outerDiv.data = `/api/projects/${projectName}/file/${currentPath}`;
//     outerDiv.innerText = fileText
//     outerDiv.addEventListener('input', startChange);
// }
//
// async function setupSlider() {
//     let projectMetadataResp = await fetch(`/api/committedchanges/${projectName}`);
//     let projectMetadataJson = await projectMetadataResp.json();
//     const queryParams = new URLSearchParams(window.location.search);
//     let maxCommitId = Math.max(...projectMetadataJson.change_ids);
//     let selectedCommitId = queryParams.get("commitId") ? queryParams.get("commitId") : maxCommitId;
//     if (maxCommitId >= 1) {
//         const commitIdInput = document.getElementById("commitIdInput");
//         commitIdInput.min = 1;
//         commitIdInput.max = maxCommitId;
//         commitIdInput.value = selectedCommitId;
//         commitIdInput.classList.remove("is-hidden")
//         commitIdInput.addEventListener("input", async () => {
//             const commitId = commitIdInput.value;
//             const queryParams = new URLSearchParams(window.location.search);
//             queryParams.set("commitId", commitId);
//
//             history.replaceState(null, null, "?"+queryParams.toString());
//             await updateFile(parseInt(commitIdInput.value))
//             return
//         });
//
//         let stepListEl = document.querySelector('#steplist')
//         for (let i = 0; i < maxCommitId; i++) {
//             let temp = document.createElement("option");
//             temp.innerHTML = i;
//             stepListEl.appendChild(temp);
//         }
//         await updateFile(parseInt(commitIdInput.value))
//     }
// }
// setupSlider();
//
// let url = `ws:/\/${window.location.host}/api/ws/committedchanges/${projectName}`;
// let c = new WebSocket(url);
//
// c.addEventListener('message', async (event) => {
//     console.log(event.data)
//     const queryParams = new URLSearchParams(window.location.search);
//     queryParams.set("commitId", event.data);
//     history.replaceState(null, null, "?"+queryParams.toString());
//     //window.location.search = searchParams.toString();
//     setupSlider()
// });
