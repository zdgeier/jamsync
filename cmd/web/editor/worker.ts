import { ChangeSet, Text } from "@codemirror/state";
import { Update } from "@codemirror/collab";

// The updates received so far (updates.length gives the current
// version)
let updates: Update[] = [];
// The current document
let doc = Text.of([""]);

let pending: ((value: any) => void)[] = [];

self.onmessage = async (event) => {
  function resp(value: any) {
    event.ports[0].postMessage(JSON.stringify(value));
  }
  let data = JSON.parse(event.data);
  if (data.type == "pullUpdates") {
    if (data.version < updates.length) {
      resp(updates.slice(data.version));
    } else {
      pending.push(resp);
    }
  } else if (data.type == "pushUpdates") {
    if (data.version != updates.length) {
      resp(false);
    } else {
      for (let update of data.updates) {
        console.log(data.updates, updates.length)
        let changes = ChangeSet.fromJSON(update.changes);
        updates.push({ changes, clientID: update.clientID });
        doc = changes.apply(doc);
      }
      resp(true);
      while (pending.length) pending.pop()!(data.updates);
    }
  } else if (data.type == "getDocument") {
    console.log("get")
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

    resp({ version: updates.length, doc: fileText});
  }
};
