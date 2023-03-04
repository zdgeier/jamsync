import {ChangeSet, Text} from "@codemirror/state"
import {Update} from "@codemirror/collab"

// The updates received so far (updates.length gives the current
// version)
let updates: Update[] = []
// The current document
let doc = Text.of([""])

//!authorityMessage

let pending: ((value: any) => void)[] = []

let ws: WebSocket;

self.onmessage = async event => {
  function resp(value: any) {
    console.log("GOT", value)
    event.ports[0].postMessage(JSON.stringify(value))
    ws.send(JSON.stringify(value))
  }
  let data = JSON.parse(event.data)
  if (data.type == "pullUpdates") {
    if (data.version < updates.length) {
    console.log("behind")
      resp(updates.slice(data.version))
    } else {
    console.log("pending")
      pending.push(resp)
    }
  } else if (data.type == "pushUpdates") {
    if (data.version != updates.length) {
      resp(false)
    } else {
      for (let update of data.updates) {
        // Convert the JSON representation to an actual ChangeSet
        // instance
        let changes = ChangeSet.fromJSON(update.changes)
        console.log("Changes", changes)
        updates.push({changes, clientID: update.clientID})
        doc = changes.apply(doc)
      }

      let body = {
        updates: data.updates,
        doc: doc.toJSON().join('\n'),
      }

      fetch(`/api/projects/${data.projectName}/file/${data.currentPath}`, {method: 'PUT', body: JSON.stringify(body)});

      resp(true)
      // Notify pending requests
      while (pending.length) pending.pop()!(data.updates)
    }
  } else if (data.type == "getDocument") {
    console.log("get")
    let url = `ws:/\/${data.host}/api/ws/committedchanges/${data.projectName}`;
    ws = new WebSocket(url);
    ws.onopen = async () => {
        let projectMetadataResp = await fetch(
        `/api/committedchanges/${data.projectName}`,
        );
        let projectMetadataJson = await projectMetadataResp.json();
        let maxCommitId = Math.max(...projectMetadataJson.change_ids);

        let fileResp = await fetch(
        `/api/projects/${data.projectName}/file/${data.currentPath}?commitId=${maxCommitId}`,
        );
        let fileText = await fileResp.text();
        doc = Text.of([fileText])
        resp({version: updates.length, doc: doc.toString()})
    }
  }
}