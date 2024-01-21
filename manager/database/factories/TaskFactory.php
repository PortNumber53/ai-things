<?php

namespace Database\Factories;

use App\Models\Task;
use Illuminate\Database\Eloquent\Factories\Factory;

/**
 * @extends \Illuminate\Database\Eloquent\Factories\Factory<\App\Models\Task>
 */
class TaskFactory extends Factory
{
    protected $model = Task::class;

    /**
     * Define the model's default state.
     *
     * @return array<string, mixed>
     */
    public function definition(): array
    {
        return [
            'task_title' => $this->faker->sentence,
            'uuid' => $this->faker->uuid,
            'prompt' => ['question' => $this->faker->sentence, 'options' => ['Option 1', 'Option 2']],
            'result' => ['answer' => 'Answer'],
            'status' => $this->faker->randomElement(['pending', 'completed']),
            'owner_id' => \App\Models\User::factory(),
            'meta' => ['extra_info' => 'Some additional information'],
        ];
    }
}
