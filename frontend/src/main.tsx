import React from "react";
import ReactDOM from "react-dom/client";
// import App from "./App.tsx";
import { createBrowserRouter, RouterProvider } from "react-router-dom";
import "./index.css";
import Root, { action as rootAction } from "./routes/root";
import ErrorPage from "./error-page";
import Task from "./routes/task";
import SignUpPage from "./SignUpPage";
import LoginPage from "./LoginPage";
import DashboardPage from "./DashboardPage";
import CreateTaskPage from "./CreateTaskPage";

const router = createBrowserRouter([
  {
    path: "/",
    element: <Root />,
    errorElement: <ErrorPage />,
    children: [
      {
        path: "tasks/{:taskId}",
        element: <Task />,
      },
      {
        path: "tasks",
        element: <Task />,
      },
      {
        path: "tasks/:taskId/edit",
      },
      {
        path: "signup",
        element: <SignUpPage />,
      },
      {
        path: "login",
        element: <LoginPage />,
      },
      {
        path: "dashboard",
        element: <DashboardPage />,
      },
      {
        path: "task",
        element: <CreateTaskPage />,
      },
      {
        path: "edit-task/:taskId",
        element: <CreateTaskPage />,
      },
    ],
  },
]);

ReactDOM.createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    <RouterProvider router={router} />
  </React.StrictMode>
);
