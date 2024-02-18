import React, { useState, useEffect } from "react";

const SignUpPage = () => {
  const [formData, setFormData] = useState({
    name: "",
    email: "",
    password: "",
    password_confirmation: "",
  });
  const [csrfToken, setCsrfToken] = useState("");

  useEffect(() => {
    async function fetchCsrfToken() {
      try {
        const response = await fetch(
          "http://localhost:8000/sanctum/csrf-cookie",
          {
            method: "GET",
          }
        );
        if (response.ok) {
          console.log("CSRF token set successfully");
        } else {
          console.error("Failed to set CSRF token");
        }
      } catch (error) {
        console.error("Failed to set CSRF token:", error.message);
      }
    }

    fetchCsrfToken();
  }, []);

  const handleChange = (e) => {
    setFormData({ ...formData, [e.target.name]: e.target.value });
  };

  const handleSubmit = async (e) => {
    e.preventDefault();
    try {
      const response = await fetch("http://localhost:8000/register", {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          "X-XSRF-TOKEN": csrfToken, // Include the CSRF token in the headers
          "X-CSRF-TOKEN": csrfToken, // Include the CSRF token in the headers
        },
        body: JSON.stringify(formData),
        credentials: "include", // Include cookies in the request
      });
      if (response.ok) {
        console.log("User registered successfully");
        // Optionally, you can redirect the user to a different page after successful registration
      } else {
        const data = await response.json();
        console.error("Failed to register user:", data.errors);
      }
    } catch (error) {
      console.error("Failed to register user:", error.message);
    }
  };

  return (
    <div>
      <h2>Sign Up</h2>
      <form onSubmit={handleSubmit}>
        <div>
          <label>Name:</label>
          <input
            type="text"
            name="name"
            value={formData.name}
            onChange={handleChange}
          />
        </div>
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
        <div>
          <label>Confirm Password:</label>
          <input
            type="password"
            name="password_confirmation"
            value={formData.password_confirmation}
            onChange={handleChange}
          />
        </div>
        <button type="submit">Sign Up</button>
      </form>
    </div>
  );
};

export default SignUpPage;
