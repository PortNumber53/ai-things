import React, { useId, useState } from "react";
import { Form } from "react-router-dom";

export default function Task() {
  const [task, setTask] = useState({
    title: "",
    uuid: "",
    prompt: "",
    status: "",
    result: "",
    meta: "",
    payload: "",
    type: "",
  });

  const handleSubmit = async (event) => {
    event.preventDefault();
    try {
      const response = await fetch("http://localhost:8000/api/tasks/", {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
        },
        body: JSON.stringify(task),
      });
      if (response.ok) {
        // Handle success, maybe show a success message or redirect
        console.log("Task saved successfully");
      } else {
        // Handle error, maybe show an error message
        console.error("Error saving task:", response.statusText);
      }
    } catch (error) {
      console.error("Error saving task:", error);
    }
  };

  const handleChange = (event) => {
    const { name, value } = event.target;
    setTask((prevTask) => ({
      ...prevTask,
      [name]: value,
    }));
  };

  const postTaskPayloadId = useId();
  const postTaskMetaId = useId();
  const postTaskPromptId = useId();
  return (
    <div id="task">
      <div>
        <div>
          <Form onSubmit={handleSubmit}>
            <div>
              <span>Title</span>
              <input
                placeholder="Title"
                aria-label="TaskTitle"
                type="text"
                name="title"
                value={task.title}
                onChange={handleChange}
              />
            </div>

            <div>
              <span>UUID</span>
              <input
                placeholder="UUID"
                aria-label="TaskUUID"
                type="text"
                name="uuid"
                value={task.uuid}
                onChange={handleChange}
              />
            </div>

            <div>
              <span htmlFor={postTaskPayloadId}>Prompt</span>
              <textarea
                aria-label="TaskPrompt"
                id={postTaskPromptId}
                name="prompt"
                rows={10}
                cols={80}
                onChange={handleChange}
              />
            </div>

            <div>
              <span htmlFor={postTaskPayloadId}>Payload</span>
              <textarea
                aria-label="TaskPayload"
                id={postTaskPayloadId}
                name="payload"
                rows={10}
                cols={80}
                onChange={handleChange}
              />
            </div>

            <div>
              <span htmlFor={postTaskPayloadId}>Meta</span>
              <textarea
                aria-label="TaskMeta"
                id={postTaskMetaId}
                name="meta"
                rows={10}
                cols={80}
                onChange={handleChange}
              />
            </div>

            <p>
              <button type="submit">Save</button>
              <button type="button">Cancel</button>
            </p>
          </Form>
        </div>
      </div>
    </div>
  );
}
