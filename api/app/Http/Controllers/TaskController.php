<?php

namespace App\Http\Controllers;

use App\Http\Requests\StoreTaskRequest;
use App\Http\Requests\UpdateTaskRequest;
use App\Models\Task;
use Inertia\Inertia;
use Inertia\Response;

class TaskController extends Controller
{
    /**
     * Display a listing of the resource.
     *
     * @return Response
     */
    public function index()
    {
        $tasks = Task::all();
        return response()->json($tasks);
    }

    public function list()
    {
        $tasks = Task::paginate(25);

        return Inertia::render(
            'Tasks/List',
            [
                'debug' => $tasks,
                'tasks' => $tasks->items(),
                'pagination' => $tasks->links(),
                'status' => session('status'),
            ]
        );
    }

    /**
     * Show the form for creating a new resource.
     */
    public function create()
    {
        //
    }

    /**
     * Store a newly created resource in storage.
     *
     * @param StoreTaskRequest $request
     * @return Response
     */
    public function store(StoreTaskRequest $request)
    {
        $task = Task::create($request->all());
        return response()->json($task, 201);
    }

    /**
     * Display the specified resource.
     *
     * @param Task $task
     * @return Response
     */
    public function show(Task $task)
    {
        return response()->json($task);
    }

    /**
     * Show the form for editing the specified resource.
     */
    public function edit(Task $task)
    {
        //
    }

    /**
     * Update the specified resource in storage.
     *
     * @param UpdateTaskRequest $request
     * @param Task $task
     * @return Response
     */
    public function update(UpdateTaskRequest $request, Task $task)
    {
        $task->update($request->all());
        return response()->json($task, 200);
    }

    /**
     * Remove the specified resource from storage.
     *
     * @param Task $task
     * @return Response
     */
    public function destroy(Task $task)
    {
        $task->delete();
        return response()->json(null, 204);
    }
}
