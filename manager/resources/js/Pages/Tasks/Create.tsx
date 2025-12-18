import AuthenticatedLayout from "@/Layouts/AuthenticatedLayout";
import { Head } from "@inertiajs/react";
import { PageProps } from "@/types";
import { useState } from "react";
import { router } from "@inertiajs/react";

interface Task {
    id: number;
    task_title: string;
    type: string;
    uuid: string;
    status: string;
    // Add more properties as needed
}

interface TaskListProps extends PageProps {
    tasks: { data: Task[]; links: { url: string; label: string }[] };
}

export default function Create({
    debug,
    auth,
    tasks,
    pagination,
}: TaskListProps) {
    console.log("DEBUG", debug);
    console.log("TASKS", tasks);
    console.log("PAGINATION", pagination);

    const [values, setValues] = useState({
        task_title: "jdslkfjsdlkfdsfsd",
        type: "tts",
        uuid: "0293840923842",
        status: "new",
        prompt: "sample prompt here",
        result: "{}",
        meta: "{}",
    });

    function handleChange(e: any) {
        const key = e.target.id;
        const value = e.target.value;
        setValues((values) => ({
            ...values,
            [key]: value,
        }));
    }

    function handleSubmit(e: any) {
        e.preventDefault();
        router.post("/tasks", values);
    }

    return (
        <AuthenticatedLayout
            user={auth.user}
            header={
                <div className="flex justify-between items-center">
                    <h2 className="font-semibold text-xl text-gray-800 dark:text-gray-200 leading-tight">
                        Create New Task
                    </h2>
                    <a
                        href="/tasks"
                        className="text-blue-500 hover:text-blue-700"
                    >
                        Back to Task List
                    </a>
                </div>
            }
        >
            <Head title="Create New Task" />

            <div className="py-12">
                <div className="max-w-7xl mx-auto sm:px-6 lg:px-8">
                    <div className="overflow-hidden shadow-sm sm:rounded-lg">
                        <div className="p-6 ">
                            <form onSubmit={handleSubmit}>
                                <label htmlFor="task_title" className="gray">
                                    Title:
                                </label>
                                <input
                                    id="task_title"
                                    value={values.task_title}
                                    onChange={handleChange}
                                />
                                <label htmlFor="type">Type:</label>
                                <input
                                    id="type"
                                    value={values.type}
                                    onChange={handleChange}
                                />
                                <label htmlFor="uuid">UUID:</label>
                                <input
                                    id="uuid"
                                    value={values.uuid}
                                    onChange={handleChange}
                                />
                                <label htmlFor="status">Status:</label>
                                <input
                                    id="status"
                                    value={values.status}
                                    onChange={handleChange}
                                />
                                <label htmlFor="prompt">Prompt:</label>
                                <input
                                    id="prompt"
                                    value={values.prompt}
                                    onChange={handleChange}
                                />
                                <label htmlFor="result">Result:</label>
                                <input
                                    id="result"
                                    value={values.result}
                                    onChange={handleChange}
                                />
                                <label htmlFor="meta">Meta:</label>
                                <input
                                    id="meta"
                                    value={values.meta}
                                    onChange={handleChange}
                                />
                                <button type="submit">Submit</button>
                            </form>
                        </div>
                    </div>
                </div>
            </div>
        </AuthenticatedLayout>
    );
}
