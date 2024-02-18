import React, { useState, useEffect } from "react";

const DashboardPage = () => {
  const [user, setUser] = useState(); // Example user object

  useEffect(() => {
    // Retrieve user data from localStorage
    const storedUser = localStorage.getItem("user");
    if (storedUser) {
      setUser(JSON.parse(storedUser));
    }
  }, []); // Empty dependency array ensures the effect runs only once on mount

  console.log("User", user);
  return (
    <div>
      <h2>Welcome, {user?.name || "User"}!</h2>
      {/* Add your dashboard content here */}
    </div>
  );
};

export default DashboardPage;
