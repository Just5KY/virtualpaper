import * as React from "react";
import SaveIcon from "@mui/icons-material/Save";
import {
  Button,
  Form,
  Loading,
  useGetManyReference,
  useGetOne,
  useNotify,
  useUpdate,
} from "react-admin";
import {
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  Grid,
  Typography,
  TextField,
  Paper,
} from "@mui/material";
import { useEffect } from "react";
import CancelIcon from "@mui/icons-material/Cancel";
import AddCircleIcon from "@mui/icons-material/AddCircle";
import DeleteOutlineIcon from "@mui/icons-material/DeleteOutline";

export interface EditLinkedDocumentsProps {
  modalOpen: boolean;
  setModalOpen: (open: boolean) => void;
  documentId: string;
}

export const EditLinkedDocuments = (props: EditLinkedDocumentsProps) => {
  const { modalOpen, setModalOpen, documentId } = props;
  const [documents, setDocuments] = React.useState<LinkedDocument[]>([]);
  const [originalDocuments, setOriginalDocuments] = React.useState<
    LinkedDocument[]
  >([]);
  const [newDocumentId, setNewDocumentId] = React.useState<string | null>(null);
  const notify = useNotify();

  const [update, { isLoading: updateLoading }] = useUpdate("documents/linked", {
    id: documentId,
    // @ts-ignore
    data: { documents: [] },
    // @ts-ignore
    previousData: { documents },
  });

  const { data: loadedDocuments, isLoading } = useGetManyReference(
    "documents/linked",
    {
      target: "id",
      id: documentId,
      meta: { originalDocuments },
    }
  );

  const { data: newDocument, isSuccess: newDocumentLoaded } = useGetOne(
    "documents",
    {
      id: newDocumentId ?? "",
      meta: {
        noVisit: true,
      },
    }
  );

  const handleSubmit = React.useCallback(
    (values: LinkedDocument[]) => {
      update("documents/linked", {
        // @ts-ignore
        data: { documents: values.map((doc) => doc.id) },
        id: documentId,
        // @ts-ignore
        meta: { documentId },
      });
    },
    [documents]
  );

  useEffect(() => {
    if (loadedDocuments && documents.length === 0) {
      setDocuments(loadedDocuments as LinkedDocument[]);
      setOriginalDocuments(loadedDocuments as LinkedDocument[]);
    }
  }, [loadedDocuments]);

  useEffect(() => {
    if (newDocument) {
      addDocumentToArray();
      setNewDocumentId(null);
    }
  }, [newDocumentLoaded, newDocument]);

  const addDocumentToArray = React.useCallback(() => {
    const docs = [...documents, { id: newDocument.id, name: newDocument.name }];
    setDocuments(docs);
  }, [newDocument, newDocumentLoaded]);

  const addDocument = (id: string) => {
    const hasDocumentLinked =
      documents.filter((document) => document.id === id).length > 0;
    if (hasDocumentLinked) {
      notify("document's already linked");
      return;
    }
    setNewDocumentId(id);
  };

  const removeDocument = (id: string) => {
    const docs = documents.filter((document) => document.id !== id);
    setDocuments(docs);
  };

  const handleCancel = () => {
    setModalOpen(false);
  };

  const handleSave = () => {
    handleSubmit(documents);
    setModalOpen(false);
  };

  if (isLoading) {
    return <Loading />;
  }

  return (
    <Dialog open={modalOpen}>
      <DialogTitle>Edit linked documents</DialogTitle>
      <Form>
        <DialogContent>
          <Typography variant="body1" sx={{ pb: 2 }}>
            Linked documents allow linking separate documents directly to each
            other. They create a one-to-one link between two documents. This
            document is currently linked to following documents:
          </Typography>
          {documents ? (
            <>
              <ShowDocumentList
                documentList={documents as unknown as LinkedDocument[]}
                removeDocument={removeDocument}
              />
              <AddDocument add={addDocument} />
            </>
          ) : (
            <Loading />
          )}
        </DialogContent>

        <DialogActions>
          <Button label={"Cancel"} onClick={handleCancel}>
            <CancelIcon />
          </Button>

          <Button
            label={updateLoading ? "Saving..." : "Save"}
            onClick={handleSave}
          >
            <SaveIcon />
          </Button>
        </DialogActions>
      </Form>
    </Dialog>
  );
};

interface LinkedDocument {
  id: string;
  name: string;
}

interface DocumentProps extends LinkedDocument {
  remove: () => void;
}

const ShowDocumentList = (props: {
  documentList: LinkedDocument[];
  removeDocument: (id: string) => void;
}) => {
  if (props.documentList.length === 0) {
    return (
      <Typography variant={"h6"}>
        No documents yet! Click below to link documents
      </Typography>
    );
  }

  return (
    <Grid>
      {props.documentList.map((document) => (
        <ShowDocument
          remove={() => props.removeDocument(document.id)}
          id={document.id}
          name={document.name}
        />
      ))}
    </Grid>
  );
};

const ShowDocument = (props: DocumentProps) => {
  return (
    <Paper elevation={3}>
      <Grid
        container
        flexDirection="row"
        sx={{ pl: 1, mt: 1, mb: 1, pt: 0, pr: 2 }}
        alignItems={"center"}
      >
        <Grid item xs={9}>
          <Typography variant="body2">
            <p style={{ textOverflow: "ellipsis", maxWidth: 300 }}>
              {props.name}
            </p>
          </Typography>
        </Grid>
        <Grid item xs={2}>
          <Button onClick={props.remove} label={"Remove"}>
            <DeleteOutlineIcon />
          </Button>
        </Grid>
      </Grid>
    </Paper>
  );
};

const AddDocument = (props: { add: (id: string) => void }) => {
  const [text, setText] = React.useState("");

  const handleAdd = () => {
    props.add(text);
  };

  return (
    <Grid container flexDirection="row" alignItems={"center"}>
      <Grid item xs={9}>
        <TextField
          id="outlined-basic"
          label="Document id"
          variant="outlined"
          value={text}
          onChange={(e) => setText(e.target.value)}
        />
      </Grid>
      <Grid item xs={2}>
        <Button onClick={handleAdd} label={"Add"}>
          <AddCircleIcon />
        </Button>
      </Grid>
    </Grid>
  );
};
