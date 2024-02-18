import { Outlet, Link } from "react-router-dom";

export default function Root() {
  return (
    <>
      <div id="sidebar">
        <h1>React Router Tasks</h1>
        <div>
          <form id="search-form" role="search">
            <input
              id="q"
              aria-label="Search task"
              placeholder="Search"
              type="search"
              name="q"
            />
            <div id="search-spinner" aria-hidden hidden={true} />
            <div className="sr-only" aria-live="polite"></div>
          </form>
          <Link to={`tasks`}>New Task</Link>
        </div>
      </div>
      <hr />
      <div id="detail">
        <Outlet />
      </div>
    </>
  );
}
