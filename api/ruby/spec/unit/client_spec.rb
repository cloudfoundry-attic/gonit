# Copyright (c) 2012 VMware, Inc.

require File.dirname(__FILE__) + '/../spec_helper'

describe GonitApi::Client do

  let(:client) { GonitApi::Client.new() }

  it "should request process status" do
    socket = mock("socket")

    client.should_receive(:connect).and_return(socket)

    socket.should_receive(:puts).with { |json|
      request = JSON.parse(json)
      request["method"].should == "API.StatusProcess"
      request["params"].should == ["gofy"]
    }

    result = {"license" => "gfl"}
    response = {"result" => result}
    socket.should_receive(:gets).and_return(response.to_json)

    socket.should_receive(:close)

    client.request("StatusProcess", "gofy").should == result
  end

  it "should request group stop" do
    socket = mock("socket")

    client.should_receive(:connect).and_return(socket)

    socket.should_receive(:puts).with { |json|
      request = JSON.parse(json)
      request["method"].should == "API.StopGroup"
      request["params"].should == ["bosh_animal_eraser"]
    }

    result = {"location" => "oleg's trunk"}
    response = {"result" => result}
    socket.should_receive(:gets).and_return(response.to_json)

    socket.should_receive(:close)

    client.stop_group("bosh_animal_eraser").should == result
  end

  it "should propagate response error" do
    socket = mock("socket")

    client.should_receive(:connect).and_return(socket)

    socket.should_receive(:puts).with { |json|
      request = JSON.parse(json)
      request["method"].should == "API.MonitorGroup"
      request["params"].should == ["dogs"]
    }

    response = {"error" => "pancakes"}
    socket.should_receive(:gets).and_return(response.to_json)

    socket.should_receive(:close)

    expect { client.monitor_group("dogs") }.to raise_error(GonitApi::RpcError)
  end

  it "should handle invalid response" do
    socket = mock("socket")

    client.should_receive(:connect).and_return(socket)

    socket.should_receive(:puts)

    socket.should_receive(:close)

    socket.should_receive(:gets).and_return("nonsense")

    expect { client.stop_all }.to raise_error(GonitApi::RpcError)
  end
end
