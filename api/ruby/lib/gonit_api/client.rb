# Copyright (c) 2012 VMware, Inc.

module GonitApi

  class RpcError < StandardError; end

  class Client

    def initialize(uri="#{ENV['HOME']}/.gonit.sock")
      @uri = URI.parse(uri)
    end

    def request(method, *args)
      socket = connect

      begin
        request = {
          "method" => "API.#{method}",
          "params" => args
        }.to_json

        socket.puts(request)

        response = socket.gets

        begin
          response = JSON.parse(response)
        rescue JSON::ParserError => e
          raise RpcError, "parsing response `#{response}': #{e.message}"
        end

        if err = response["error"]
          raise RpcError, err
        end

        response["result"]
      ensure
        socket.close
      end
    end

    def method_missing(name, *args)
      name = name.to_s.split("_").map { |s| s.capitalize }.join("")
      request(name, *args)
    end

    private

    def connect
      case @uri.scheme
      when "tcp"
        TCPSocket.new(@uri.host, @uri.port)
      else
        UNIXSocket.new(@uri.path)
      end
    end

  end
end
