import AuthenticatedLayout from "@/Layouts/AuthenticatedLayout";
import { Head } from "@inertiajs/react";
import { PageProps } from "@/types";

interface Task {
    id: number;
    task_title: string;
    uuid: string;
    status: string;
    // Add more properties as needed
}

interface TaskListProps extends PageProps {
    tasks: { data: Task[]; links: { url: string; label: string }[] };
}

export default function Task({
    debug,
    auth,
    tasks,
    pagination,
}: TaskListProps) {
    console.log("DEBUG", debug);
    console.log("TASKS", tasks);
    console.log("PAGINATION", pagination);
    return (
        <AuthenticatedLayout
            user={auth.user}
            header={
                <h2 className="font-semibold text-xl text-gray-800 dark:text-gray-200 leading-tight">
                    Task List
                </h2>
            }
        >
            <Head title="Task List" />

            <div className="py-12">
                <div className="max-w-7xl mx-auto sm:px-6 lg:px-8">
                    <div className="bg-white dark:bg-gray-800 overflow-hidden shadow-sm sm:rounded-lg">
                        <div className="p-6 text-gray-900 dark:text-gray-100">
                            <table className="min-w-full">
                                <thead>
                                    <tr>
                                        <th className="py-2">Task Title</th>
                                        <th className="py-2">UUID</th>
                                        <th className="py-2">Status</th>
                                        {/* Add more columns as needed */}
                                        <th className="py-2">Actions</th>
                                    </tr>
                                </thead>
                                <tbody>
                                    {tasks.data &&
                                        tasks.data.map((task) => (
                                            <tr key={task.id}>
                                                <td className="py-2">
                                                    {task.task_title}
                                                </td>
                                                <td className="py-2">
                                                    {task.uuid}
                                                </td>
                                                <td className="py-2">
                                                    {task.status}
                                                </td>
                                                {/* Add more cells as needed */}
                                                <td className="p-2">actions</td>
                                            </tr>
                                        ))}
                                </tbody>
                                <tfoot>
                                    <tr>
                                        <td colSpan={4}>
                                            {tasks.links &&
                                                tasks.links.map(
                                                    (link, index) => (
                                                        <div key={index}>
                                                            <a href={link.url}>
                                                                {link.label}
                                                            </a>
                                                        </div>
                                                    )
                                                )}
                                        </td>
                                    </tr>
                                </tfoot>
                            </table>
                        </div>
                    </div>
                </div>
            </div>
        </AuthenticatedLayout>
    );
}
