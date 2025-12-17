<?php

namespace App\Console\Commands;

use Illuminate\Console\Command;
use Illuminate\Support\Facades\Http;
use App\Support\ExtraEnv;


class AgentCreateUnitTest extends Command
{
    /**
     * The name and signature of the console command.
     *
     * @var string
     */
    protected $signature = 'agent:CreateUnitTest {file}';

    /**
     * The console command description.
     *
     * @var string
     */
    protected $description = 'Creates a unit test for the given file';

    /**
     * Execute the console command.
     */
    public function handle()
    {
        $file = $this->argument('file');

        var_dump($file);
        var_dump(base_path());

        $test_file = '';
        // generate the file path for the unit test
        if (strpos($file, 'app/') === 0) {
            $test_file = 'tests/' . substr($file, 4);
            $absolute_file = base_path() . '/' . $file;
            var_dump($absolute_file);
        }

        echo 'test_file: ';
        var_dump($test_file);
        $absolute_test_file = base_path() . '/' . $test_file;
        var_dump($absolute_test_file);

        // check if application file exists and test file doesn't
        if ($app_file_exists = is_file($absolute_file) && (!is_file($test_file))) {
            //
            $this->info('Can create test file');
        } else {
            $this->alert('Something wrong');
        }

        // $response = Http::get('https://portnumber53.com');

        $application_file = <<<APPLICATION_FILE
<?php
use Illuminate\\Support\\Facades\\Hash;

class PasswordVerifier
{
    private \$passwordHasher;

    public function __construct()
    {
        \$this->passwordHasher = Hash::create();
    }

    /**
     * Verify a user's password.
     *
     * @param string \$userPassword The user's password to verify.
     * @param string \$hashedPassword The hashed password to compare with.
     *
     * @return bool True if the passwords match, false otherwise.
     */
    public function verifyPassword(\$userPassword, \$hashedPassword)
    {
        return \$this->passwordHasher::check(\$userPassword, \$hashedPassword);
    }
}
APPLICATION_FILE;

var_dump($app_file_exists);
        if ($app_file_exists === true) {
            $application_file_contents = file_get_contents($absolute_file);
            // var_dump($application_file);
            // echo "\n\n";
          }

        ExtraEnv::load();
        $apiKey = env('PORTNUMBER53_API_KEY');
        if (empty($apiKey)) {
            $this->error('Missing PORTNUMBER53_API_KEY (set it in .env or _extra_env).');
            return 1;
        }

        $response = Http::timeout(300)->withHeaders([
            'X-API-key' => $apiKey,
        ])->post('https://ollama.portnumber53.com/api/generate', [
            'model' => 'llama3.1:8b',
            'stream' => false,
            'prompt' => <<<PROMPT
SYSTEM """
You are a professional PHP developer.
Your response must be just the raw source code for the unit test file, do not include an explanation ever. do include markdown notations.
Example unit test response for a PHP class with a method that requests a row by ID from the database:
<?php
use PHPUnit\Framework\TestCase;
use App\Class\Utility\Database;

class DatabaseTest extends TestCase {
    private \$db;

    protected function setUp(): void {
        // Set up a mock PDO instance and pass it to the Database class
        \$pdo = \$this->createMock(PDO::class);
        \$this->db = new Database(\$pdo);
    }

    public function testGetRowByIdReturnsCorrectRow() {
        // Create a mock statement
        \$statement = \$this->createMock(PDOStatement::class);
        \$statement->expects(\$this->once())
                  ->method('bindParam')
                  ->with(':id', 1, PDO::PARAM_INT);
        \$statement->expects(\$this->once())
                  ->method('execute');
        \$statement->expects(\$this->once())
                  ->method('fetch')
                  ->willReturn(['id' => 1, 'name' => 'Test']);

        // Configure the mock PDO to return the mock statement
        \$this->db->pdo->expects(\$this->once())
                      ->method('prepare')
                      ->with('SELECT * FROM your_table_name WHERE id = :id')
                      ->willReturn(\$statement);

        \$result = \$this->db->getRowById('your_table_name', 1);

        \$this->assertSame(['id' => 1, 'name' => 'Test'], \$result);
    }

    public function testGetRowByIdThrowsExceptionOnQueryError() {
        // Configure the mock PDO to throw an exception on prepare
        \$this->db->pdo->expects(\$this->once())
                      ->method('prepare')
                      ->will(\$this->throwException(new PDOException('Query error')));

        \$this->expectException(Exception::class);
        \$this->expectExceptionMessage('Query error');

        \$this->db->getRowById('your_table_name', 1);
    }
}
"""
USER """
Write a php unit test for this file:
$application_file_contents
"""
PROMPT,
        ]);
        
        $response_json = $response->json();
        $body_response = $response_json['response'];

        // remove PHP markdow in first line
        $array_body = explode("\n", $body_response);
        if (trim($array_body[0]) === '```php') {
            $this->info('remove first line');
            $body_response = substr($body_respose, strlen($array_body[0]));
         }
 
        print_r($body_response);

        echo "\n\n";
        var_dump(base_path());
    }
}
