import React, { useState } from "react";
import { redirect } from "react-router-dom";

const LoginPage = () => {
  const [formData, setFormData] = useState({
    email: "",
    password: "",
  });

  const [user, setUser] = useState(null);

  const handleChange = (e) => {
    setFormData({ ...formData, [e.target.name]: e.target.value });
  };

  const handleSubmit = async (e) => {
    e.preventDefault();
    try {
      // Fetch CSRF token from the backend
      await fetch("http://localhost:8000/sanctum/csrf-cookie", {
        method: "GET",
        mode: "cors",
        credentials: "include",
      });

      // Send login request with formData
      const response = await fetch("http://localhost:8000/login", {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
        },
        body: JSON.stringify(formData),
        credentials: "include", // Include cookies in the request
      });

      if (response.ok) {
        console.log("User logged in successfully");
        const responseData = await response.json();
        console.log("responseData", responseData);

        // Set user data in state and localStorage
        setUser(responseData.user);
        localStorage.setItem("user", JSON.stringify(responseData.user));

        console.log("USER:", user);
        console.log("redirect_to:", responseData.redirect_to);

        // Redirect user if needed
        if (responseData.redirect_to) {
          window.location.href = responseData.redirect_to;
        }
      } else {
        console.error("Failed to login:", response.statusText);
      }
    } catch (error) {
      console.error("Failed to login:", error.message);
    }
  };

  return (
    <div>
      <h2>Login</h2>
      <form onSubmit={handleSubmit}>
        <div>
          <label>Email:</label>
          <input
            type="email"
            name="email"
            value={formData.email}
            onChange={handleChange}
          />
        </div>
        <div>
          <label>Password:</label>
          <input
            type="password"
            name="password"
            value={formData.password}
            onChange={handleChange}
          />
        </div>
        <button type="submit">Login</button>
      </form>
    </div>
  );
};

export default LoginPage;
