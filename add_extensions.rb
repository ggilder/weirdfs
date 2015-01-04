#!/usr/bin/env ruby

require 'fileutils'

# Basic script to add file extensions to older Mac files
USAGE = <<EOD
add_extensions.rb [--dry-run] [PATH]

`--dry-run` prints the actions to be taken without performing them.
PATH is optional; defaults to current directory.
EOD

if ARGV.delete('--help') || ARGV.delete('-h')
  puts USAGE
  exit
end

dry_run = ARGV.delete('--dry-run')
path = ARGV.shift || Dir.pwd
path = File.expand_path(path)

puts "Scanning #{path} for files without extensions..."

files = Dir[File.join(path, '**/*')].select do |file|
  File.extname(file) == '' && File.file?(file)
end

known_extensions = {
  'AIFF' => '.aif',
  'Sd2f' => '.sd2',
}

files.each do |file|
  type = `GetFileInfo -t "#{file}"`.chomp.gsub('"', '')
  extension = known_extensions[type]
  if extension
    puts %(Renaming "#{file}" to "#{file}#{extension}".)
    unless dry_run
      FileUtils.mv(file, "#{file}#{extension}", verbose: true)
    end
  else
    $stderr.puts %(Don't know what extension to use for "#{type}" file type.)
  end
end

puts "Done."
