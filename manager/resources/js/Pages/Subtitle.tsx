import { useState, useEffect } from "react";
import { useForm } from "react-hook-form";
import AuthenticatedLayout from "@/Layouts/AuthenticatedLayout";
import { Head } from "@inertiajs/react";
import { PageProps } from "@/types";

export default function Subtitle({ auth, content }: PageProps) {
    const { register, handleSubmit, setValue } = useForm();

    useEffect(() => {
        if (content) {
            setValue("originalContent", content.sentences); // Pre-fill original content
        }
    }, [content]);

    const onSubmit = (data) => {
        // Handle form submission
        console.log(data);
        // You can send the data to your Laravel backend here
    };

    return (
        <AuthenticatedLayout
            user={auth.user}
            header={
                <h2 className="font-semibold text-xl text-gray-800 dark:text-gray-200 leading-tight">
                    Subtitle
                </h2>
            }
        >
            <Head title="Subtitle" />

            <div className="py-12">
                <div className="max-w-7xl mx-auto sm:px-6 lg:px-8">
                    <div className="bg-white dark:bg-gray-800 overflow-hidden shadow-sm sm:rounded-lg">
                        <div className="p-6 text-gray-900 dark:text-gray-100">
                            <form
                                onSubmit={handleSubmit(onSubmit)}
                                className="flex flex-wrap"
                            >
                                <div className="w-full md:w-1/2 md:pr-2 mb-4">
                                    <label className="block text-gray-700 text-sm font-bold mb-2">
                                        Original Content
                                    </label>
                                    <textarea
                                        name="originalContent"
                                        className="w-full border rounded-md py-2 px-3 text-gray-700 leading-tight focus:outline-none focus:shadow-outline"
                                        rows="4"
                                        {...register("originalContent")}
                                        readOnly
                                    />
                                </div>
                                <div className="w-full md:w-1/2 md:pl-2 mb-4">
                                    <label className="block text-gray-700 text-sm font-bold mb-2">
                                        Editable Content
                                    </label>
                                    <textarea
                                        name="editableContent"
                                        className="w-full border rounded-md py-2 px-3 text-gray-700 leading-tight focus:outline-none focus:shadow-outline"
                                        rows="4"
                                        {...register("editableContent")}
                                    />
                                </div>
                                <div className="w-full flex justify-end">
                                    <button
                                        type="submit"
                                        className="bg-blue-500 hover:bg-blue-700 text-white font-bold py-2 px-4 rounded focus:outline-none focus:shadow-outline"
                                    >
                                        Save
                                    </button>
                                </div>
                            </form>
                        </div>
                    </div>
                </div>
            </div>
        </AuthenticatedLayout>
    );
}
