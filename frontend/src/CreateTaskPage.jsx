import React, { useState, useEffect } from "react";
import { useNavigate, useParams } from "react-router-dom";
import { v4 as uuidv4 } from "uuid";

const CreateTaskPage = () => {
  const navigate = useNavigate();

  const [formData, setFormData] = useState({
    task_title: "",
    owner_id: 2,
    uuid: uuidv4(),
    prompt: "",
    status: "draft",
    result: "",
    meta: "",
    payload: "",
    type: "",
  });

  const { taskId } = useParams();

  useEffect(() => {
    if (taskId) {
      // Fetch existing task data if editing
      const fetchTaskData = async () => {
        try {
          const response = await fetch(
            `http://localhost:8000/api/tasks/${taskId}`
          );
          if (response.ok) {
            const taskData = await response.json();
            setFormData(taskData);
          } else {
            throw new Error("Failed to fetch task data");
          }
        } catch (error) {
          console.error("Failed to fetch task data:", error.message);
        }
      };

      fetchTaskData();
    }
  }, [taskId]); // Fetch task data only when taskId changes

  const handleChange = (e) => {
    setFormData({ ...formData, [e.target.name]: e.target.value });
  };

  const handleSubmit = async (e) => {
    e.preventDefault();
    try {
      let url = "http://localhost:8000/api/tasks";
      let method = "POST";

      if (taskId) {
        url += `/${taskId}`;
        method = "PUT";
      }

      // Send formData to the API endpoint
      const response = await fetch(url, {
        method: method,
        mode: "cors",
        credentials: "include",
        headers: {
          "Content-Type": "application/json",
        },
        body: JSON.stringify(formData),
      });

      if (response.ok) {
        if (taskId) {
          console.log("Task updated successfully");
        } else {
          const responseData = await response.json();

          console.log("Task created successfully", responseData);
          if (responseData.id) {
            // TO-DO: set taskId to response.id value so the page switches to editing it
            console.log("will redirect");

            navigate(`/edit-task/${responseData.id}`);
          }
        }
      } else {
        throw new Error(
          `Failed to ${taskId ? "update" : "create"} task: ${
            response.statusText
          }`
        );
      }
    } catch (error) {
      console.error(
        `Failed to ${taskId ? "update" : "create"} task:`,
        error.message
      );
    }
  };

  return (
    <div>
      <h2>{taskId ? "Edit Task" : "Create Task"}</h2>
      <form onSubmit={handleSubmit}>
        <div>
          <label>Title:</label>
          <input
            type="text"
            name="task_title"
            value={formData.task_title}
            onChange={handleChange}
          />
        </div>
        <div>
          <label>UUID:</label>
          <input
            type="text"
            name="uuid"
            value={formData.uuid}
            onChange={handleChange}
          />
        </div>
        <div>
          <label>Status:</label>
          <input
            type="text"
            name="status"
            value={formData.status}
            onChange={handleChange}
          />
        </div>
        <div>
          <label>OwnerId:</label>
          <input
            type="number"
            name="owner_id"
            value={formData.owner_id}
            onChange={handleChange}
          />
        </div>

        <div style={{ width: "100%", marginBottom: "1rem" }}>
          <label>Prompt:</label>
          <textarea
            style={{ width: "100%", height: "8rem", resize: "vertical" }}
            name="prompt"
            value={formData.prompt}
            onChange={handleChange}
          />
        </div>
        {/* Add input fields for other Task properties as needed */}
        <button type="submit">{taskId ? "Update" : "Create"} Task</button>
      </form>
    </div>
  );
};

export default CreateTaskPage;
